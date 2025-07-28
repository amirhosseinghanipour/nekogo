package core

import (
	"fmt"
	"sync/atomic"
	"time"
)

var (
	bytesSentLastSecond     int64
	bytesReceivedLastSecond int64
)

var Stats struct {
	SentRate     string
	ReceivedRate string
}

func init() {
	// This goroutine calculates the transfer rates every second.
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// Get the number of bytes transferred in the last second.
			sent := atomic.SwapInt64(&bytesSentLastSecond, 0)
			received := atomic.SwapInt64(&bytesReceivedLastSecond, 0)

			// Update the public stats with formatted strings (KB/s).
			Stats.SentRate = formatRate(sent)
			Stats.ReceivedRate = formatRate(received)
		}
	}()
}

func AddBytesSent(n int64) {
	atomic.AddInt64(&bytesSentLastSecond, n)
}

func AddBytesReceived(n int64) {
	atomic.AddInt64(&bytesReceivedLastSecond, n)
}

// formatRate converts a byte count into a human-readable rate string.
func formatRate(bytes int64) string {
	kbs := float64(bytes) / 1024.0
	if kbs < 1024.0 {
		return fmt.Sprintf("%.2f KB/s", kbs)
	}
	mbs := kbs / 1024.0
	return fmt.Sprintf("%.2f MB/s", mbs)
}