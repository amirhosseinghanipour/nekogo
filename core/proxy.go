package core

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"

	"github.com/armon/go-socks5"
)

func StartProxy(proxyType, addr string) error {
	switch proxyType {
	case "socks5":
		conf := &socks5.Config{}
		srv, err := socks5.New(conf)
		if err != nil {
			return fmt.Errorf("failed to create SOCKS5 server: %w", err)
		}
		go func() {
			log.Printf("SOCKS5 proxy listening on %s", addr)
			if err := srv.ListenAndServe("tcp", addr); err != nil {
				log.Printf("SOCKS5 proxy error: %v", err)
			}
		}()
	case "http":
		proxy := &httputil.ReverseProxy{Director: func(req *http.Request) {}}
		go func() {
			log.Printf("HTTP proxy listening on %s", addr)
			if err := http.ListenAndServe(addr, proxy); err != nil {
				log.Printf("HTTP proxy error: %v", err)
			}
		}()
	case "https":
		// HTTPS proxy (CONNECT tunnel, no MITM)
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				// Handle CONNECT
				log.Printf("HTTPS CONNECT %s", r.Host)
				conn, err := net.Dial("tcp", r.Host)
				if err != nil {
					http.Error(w, "Failed to connect to target", http.StatusServiceUnavailable)
					return
				}
				hj, ok := w.(http.Hijacker)
				if !ok {
					http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
					return
				}
				clientConn, _, err := hj.Hijack()
				if err != nil {
					http.Error(w, "Hijack failed", http.StatusInternalServerError)
					return
				}
				clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
				go func() { io.Copy(conn, clientConn) }()
				go func() { io.Copy(clientConn, conn) }()
			} else {
				w.WriteHeader(http.StatusMethodNotAllowed)
			}
		})
		go func() {
			log.Printf("HTTPS proxy (CONNECT) listening on %s", addr)
			if err := http.ListenAndServe(addr, handler); err != nil {
				log.Printf("HTTPS proxy error: %v", err)
			}
		}()
	default:
		return fmt.Errorf("unsupported proxy type: %s", proxyType)
	}
	return nil
}
