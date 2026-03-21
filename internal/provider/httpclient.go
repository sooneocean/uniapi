package provider

import (
	"net"
	"net/http"
	"time"
)

// DefaultHTTPClient returns an HTTP client optimized for API calls
// with connection pooling and keep-alive.
func DefaultHTTPClient() *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	}
	return &http.Client{
		Timeout:   120 * time.Second,
		Transport: transport,
	}
}
