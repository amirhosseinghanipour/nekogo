package core

import "sync/atomic"

var Stats struct {
	BytesSent     int64
	BytesReceived int64
}

func AddBytesSent(n int64) {
	atomic.AddInt64(&Stats.BytesSent, n)
}

func AddBytesReceived(n int64) {
	atomic.AddInt64(&Stats.BytesReceived, n)
}