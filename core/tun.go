package core

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os/exec"
	"runtime"

	"github.com/amirhosseinghanipour/nekogo/config"
	ss "github.com/shadowsocks/go-shadowsocks2/core"
	"github.com/songgao/water"
	"golang.org/x/net/proxy"
)

type Forwarder interface {
	ForwardTCP(pkt []byte) error
	ForwardUDP(pkt []byte) error
}

type ShadowsocksForwarder struct {
	Server config.ServerConfig
	Cipher ss.Cipher
}

func NewShadowsocksForwarder(server config.ServerConfig) (*ShadowsocksForwarder, error) {
	cipher, err := ss.PickCipher(server.Method, nil, server.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	return &ShadowsocksForwarder{Server: server, Cipher: cipher}, nil
}

// SOCKS5 Forwarder

type Socks5Forwarder struct {
	Server config.ServerConfig
}

func NewSocks5Forwarder(server config.ServerConfig) (*Socks5Forwarder, error) {
	return &Socks5Forwarder{Server: server}, nil
}

func (s *Socks5Forwarder) ForwardTCP(pkt []byte) error {
	if len(pkt) < 20 {
		return fmt.Errorf("packet too short")
	}
	ihl := int(pkt[0]&0x0F) * 4
	if len(pkt) < ihl+20 {
		return fmt.Errorf("not enough data for TCP header")
	}
	dstIP := net.IP(pkt[16:20])
	dstPort := binary.BigEndian.Uint16(pkt[ihl+2 : ihl+4])
	log.Printf("TUN TCP -> %s:%d (SOCKS5)", dstIP, dstPort)

	proxyAddr := fmt.Sprintf("%s:%d", s.Server.Address, s.Server.Port)
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 dialer: %w", err)
	}
	conn, err := dialer.Dial("tcp", fmt.Sprintf("%s:%d", dstIP.String(), dstPort))
	if err != nil {
		return fmt.Errorf("failed to dial via SOCKS5: %w", err)
	}
	defer conn.Close()
	ihlTCP := ihl + 20
	if len(pkt) > ihlTCP {
		payload := pkt[ihlTCP:]
		n, err := conn.Write(payload)
		if err != nil {
			return fmt.Errorf("failed to write payload: %w", err)
		}
		AddBytesSent(int64(n))
	}
	return nil
}

func (s *Socks5Forwarder) ForwardUDP(pkt []byte) error {
	if len(pkt) < 20 {
		return fmt.Errorf("packet too short")
	}
	ihl := int(pkt[0]&0x0F) * 4
	if len(pkt) < ihl+8 {
		return fmt.Errorf("not enough data for UDP header")
	}
	dstIP := net.IP(pkt[16:20])
	dstPort := binary.BigEndian.Uint16(pkt[ihl+2 : ihl+4])
	log.Printf("TUN UDP -> %s:%d (SOCKS5)", dstIP, dstPort)

	proxyAddr := fmt.Sprintf("%s:%d", s.Server.Address, s.Server.Port)
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
	if err != nil {
		return fmt.Errorf("failed to create SOCKS5 dialer for UDP associate: %w", err)
	}
	conn, err := dialer.Dial("tcp", proxyAddr) // Connect to the SOCKS server
	if err != nil {
		return fmt.Errorf("failed to dial SOCKS5 server for UDP associate: %w", err)
	}
	defer conn.Close()

	// 1. Send UDP ASSOCIATE command
	b := []byte{0x05, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if _, err := conn.Write(b); err != nil {
		return fmt.Errorf("failed to write UDP associate request: %w", err)
	}

	// 2. Read the server's reply
	resp := make([]byte, 256)
	n, err := conn.Read(resp)
	if err != nil {
		return fmt.Errorf("failed to read UDP associate reply: %w", err)
	}
	if n < 10 || resp[0] != 0x05 || resp[1] != 0x00 {
		return fmt.Errorf("UDP associate failed, server response: %v", resp[:n])
	}
	bndPort := binary.BigEndian.Uint16(resp[n-2:])
	bndAddr := net.IP(resp[4 : n-2])

	// 3. Send the UDP datagram to the address we got from the server
	udpConn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: bndAddr, Port: int(bndPort)})
	if err != nil {
		return fmt.Errorf("failed to dial UDP relay: %w", err)
	}
	defer udpConn.Close()

	// 4. Construct the SOCKS5 UDP request packet
	udpReq := []byte{0x00, 0x00, 0x00, 0x01} // RSV, FRAG, ATYP (IPv4)
	udpReq = append(udpReq, dstIP.To4()...)
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, dstPort)
	udpReq = append(udpReq, portBytes...)
	udpReq = append(udpReq, pkt[ihl+8:]...) // Append the original UDP payload

	if _, err := udpConn.Write(udpReq); err != nil {
		return fmt.Errorf("failed to write UDP packet to relay: %w", err)
	}
	AddBytesSent(int64(len(udpReq)))
	return nil
}

// HTTP Forwarder (CONNECT only, for HTTPS)
type HttpForwarder struct {
	Server config.ServerConfig
}

func NewHttpForwarder(server config.ServerConfig) (*HttpForwarder, error) {
	return &HttpForwarder{Server: server}, nil
}

func (h *HttpForwarder) ForwardTCP(pkt []byte) error {
	if len(pkt) < 20 {
		return fmt.Errorf("packet too short")
	}
	ihl := int(pkt[0]&0x0F) * 4
	if len(pkt) < ihl+20 {
		return fmt.Errorf("not enough data for TCP header")
	}
	dstIP := net.IP(pkt[16:20])
	dstPort := binary.BigEndian.Uint16(pkt[ihl+2 : ihl+4])
	log.Printf("TUN TCP -> %s:%d (HTTP/HTTPS)", dstIP, dstPort)

	proxyAddr := fmt.Sprintf("%s:%d", h.Server.Address, h.Server.Port)
	conn, err := net.Dial("tcp", proxyAddr)
	if err != nil {
		return fmt.Errorf("failed to dial HTTP proxy: %w", err)
	}
	defer conn.Close()
	// Send CONNECT request
	connectReq := fmt.Sprintf("CONNECT %s:%d HTTP/1.1\r\nHost: %s:%d\r\n\r\n", dstIP.String(), dstPort, dstIP.String(), dstPort)
	if _, err := conn.Write([]byte(connectReq)); err != nil {
		return fmt.Errorf("failed to send CONNECT: %w", err)
	}
	// Read response (very basic, not robust)
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read CONNECT response: %w", err)
	}
	if n < 12 || string(buf[:12]) != "HTTP/1.1 200" {
		return fmt.Errorf("proxy CONNECT failed: %s", string(buf[:n]))
	}
	ihlTCP := ihl + 20
	if len(pkt) > ihlTCP {
		payload := pkt[ihlTCP:]
		n, err := conn.Write(payload)
		if err != nil {
			return fmt.Errorf("failed to write payload: %w", err)
		}
		AddBytesSent(int64(n))
	}
	return nil
}

func (h *HttpForwarder) ForwardUDP(pkt []byte) error {
	log.Println("UDP forwarding is not supported by HTTP proxies.")
	return nil
}

// WriteAddr writes the Shadowsocks address format
func WriteAddr(conn net.Conn, ip net.IP, port uint16) error {
	if ip.To4() != nil {
		if _, err := conn.Write([]byte{0x01}); err != nil {
			return err
		}
		if _, err := conn.Write(ip.To4()); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("unsupported address type")
	}
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, port)
	_, err := conn.Write(portBytes)
	return err
}

func (s *ShadowsocksForwarder) ForwardTCP(pkt []byte) error {
	if len(pkt) < 20 {
		return fmt.Errorf("packet too short")
	}
	ihl := int(pkt[0]&0x0F) * 4
	if len(pkt) < ihl+20 {
		return fmt.Errorf("not enough data for TCP header")
	}
	dstIP := net.IP(pkt[16:20])
	dstPort := binary.BigEndian.Uint16(pkt[ihl+2 : ihl+4])
	log.Printf("TUN TCP -> %s:%d", dstIP, dstPort)

	ssAddr := fmt.Sprintf("%s:%d", s.Server.Address, s.Server.Port)
	rawConn, err := net.Dial("tcp", ssAddr)
	if err != nil {
		return fmt.Errorf("failed to dial Shadowsocks: %w", err)
	}
	defer rawConn.Close()
	conn := s.Cipher.StreamConn(rawConn)

	if err := WriteAddr(conn, dstIP, dstPort); err != nil {
		return fmt.Errorf("failed to write addr: %w", err)
	}
	ihlTCP := ihl + 20
	if len(pkt) > ihlTCP {
		payload := pkt[ihlTCP:]
		n, err := conn.Write(payload)
		if err != nil {
			return fmt.Errorf("failed to write payload: %w", err)
		}
		AddBytesSent(int64(n))
	}
	return nil
}

func (s *ShadowsocksForwarder) ForwardUDP(pkt []byte) error {
	if len(pkt) < 20 {
		return fmt.Errorf("packet too short")
	}
	ihl := int(pkt[0]&0x0F) * 4
	if len(pkt) < ihl+8 {
		return fmt.Errorf("not enough data for UDP header")
	}
	dstIP := net.IP(pkt[16:20])
	dstPort := binary.BigEndian.Uint16(pkt[ihl+2 : ihl+4])
	log.Printf("TUN UDP -> %s:%d", dstIP, dstPort)

	ssAddr := fmt.Sprintf("%s:%d", s.Server.Address, s.Server.Port)
	c, err := net.Dial("udp", ssAddr)
	if err != nil {
		return fmt.Errorf("failed to dial UDP for Shadowsocks: %w", err)
	}
	defer c.Close()

	pc, ok := c.(net.PacketConn)
	if !ok {
		return fmt.Errorf("connection does not implement net.PacketConn")
	}
	pc = s.Cipher.PacketConn(pc)

	tgt := &net.UDPAddr{IP: dstIP, Port: int(dstPort)}
	payload := pkt[ihl+8:]
	_, err = pc.WriteTo(payload, tgt)
	if err != nil {
		return fmt.Errorf("failed to write payload to SS UDP: %w", err)
	}
	AddBytesSent(int64(len(payload)))
	return nil
}

func StartTUNWithConfig(cfg *config.AppConfig, stopChan <-chan struct{}) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	active := cfg.Servers[cfg.ActiveIndex]
	var forwarder Forwarder
	var err error

	switch active.Type {
	case "shadowsocks":
		forwarder, err = NewShadowsocksForwarder(active)
	case "socks5":
		forwarder, err = NewSocks5Forwarder(active)
	// VLESS, VMess, and Trojan require a full V2Ray-core implementation, which is a very large project.
	// This TUN implementation will forward standard protocols through Shadowsocks/SOCKS5.
	default:
		return fmt.Errorf("unsupported server type for TUN mode: %s", active.Type)
	}
	if err != nil {
		return err
	}

	ifce, err := setupTUN()
	if err != nil {
		return err
	}
	defer ifce.Close()
	log.Printf("TUN interface created: %s", ifce.Name())

	packetChan := make(chan []byte, 100)
	for i := 0; i < 10; i++ {
		go packetWorker(ifce, forwarder, packetChan)
	}

	go func() {
		buf := make([]byte, 1500)
		for {
			select {
			case <-stopChan:
				return
			default:
				n, err := ifce.Read(buf)
				if err != nil {
					return
				}
				packet := make([]byte, n)
				copy(packet, buf[:n])
				packetChan <- packet
			}
		}
	}()

	<-stopChan
	log.Println("TUN mode stopped.")
	return nil
}


func packetWorker(ifce TUNDevice, forwarder Forwarder, packetChan <-chan []byte) {
	for packet := range packetChan {
		AddBytesReceived(int64(len(packet)))
		if len(packet) < 20 {
			continue // Not a valid IP packet
		}
		proto := packet[9] // IPv4 protocol field
		switch proto {
		case 1: // ICMP
			handleICMP(ifce, packet)
		case 6: // TCP
			if err := forwarder.ForwardTCP(packet); err != nil {
				log.Printf("TCP forwarding error: %v", err)
			}
		case 17: // UDP
			if err := forwarder.ForwardUDP(packet); err != nil {
				log.Printf("UDP forwarding error: %v", err)
			}
		}
	}
}

func handleICMP(ifce TUNDevice, pkt []byte) {
	if len(pkt) < 28 {
		return
	}
	if pkt[20] == 8 && pkt[21] == 0 {
		log.Println("TUN ICMP -> Echo Request received (Ping)")
		replyPkt := make([]byte, len(pkt))
		copy(replyPkt, pkt)

		copy(replyPkt[12:16], pkt[16:20])
		copy(replyPkt[16:20], pkt[12:16])
		replyPkt[20] = 0
		replyPkt[10] = 0
		replyPkt[22] = 0
		binary.BigEndian.PutUint16(replyPkt[22:], checksum(replyPkt[20:]))
		binary.BigEndian.PutUint16(replyPkt[10:], checksum(replyPkt[:20]))

		if _, err := ifce.Write(replyPkt); err != nil {
			log.Printf("Failed to write ICMP echo reply: %v", err)
		} else {
			AddBytesSent(int64(len(replyPkt)))
		}
	}
}

func checksum(data []byte) uint16 {
	var sum uint32
	for i := 0; i+1 < len(data); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(data[i:]))
	}
	if len(data)%2 == 1 {
		sum += uint32(data[len(data)-1])
	}
	for sum>>16 > 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return uint16(^sum)
}

func setupTUN() (TUNDevice, error) {
	cfg := water.Config{DeviceType: water.TUN}
	cfg.Name = "nekogo-tun"
	ifce, err := water.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create TUN interface: %w", err)
	}
	switch runtime.GOOS {
	case "linux":
		if err := exec.Command("sudo", "ip", "addr", "add", "10.0.85.2/24", "dev", ifce.Name()).Run(); err != nil {
			log.Printf("Error setting IP address: %v", err)
		}
		if err := exec.Command("sudo", "ip", "link", "set", "dev", ifce.Name(), "up").Run(); err != nil {
			log.Printf("Error bringing up interface: %v", err)
		}
		if err := exec.Command("sudo", "ip", "route", "add", "default", "dev", ifce.Name()).Run(); err != nil {
			log.Printf("Error setting default route: %v", err)
		}
	case "darwin":
		if err := exec.Command("sudo", "ifconfig", ifce.Name(), "10.0.85.2", "10.0.85.1", "up").Run(); err != nil {
			return nil, fmt.Errorf("failed to setup TUN interface on macOS: %w", err)
		}
		if err := exec.Command("sudo", "route", "add", "default", "10.0.85.1").Run(); err != nil {
			return nil, fmt.Errorf("failed to set default route on macOS: %w", err)
		}
	case "windows":
		if err := exec.Command("netsh", "interface", "ip", "set", "address", fmt.Sprintf("name=\"%s\"", ifce.Name()), "static", "10.0.85.2", "255.255.255.0").Run(); err != nil {
			return nil, fmt.Errorf("failed to setup TUN interface on Windows: %w", err)
		}
		if err := exec.Command("netsh", "interface", "ip", "set", "dns", fmt.Sprintf("name=\"%s\"", ifce.Name()), "static", "8.8.8.8").Run(); err != nil {
			log.Printf("Could not set DNS on Windows, this is not a fatal error: %v", err)
		}
	}
	return ifce, nil
}

type TUNDevice interface {
	Name() string
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
}
