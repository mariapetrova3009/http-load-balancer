package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// NewReverseProxy builds a reverse proxy to a single backend.
func NewReverseProxy(target *url.URL) *httputil.ReverseProxy {
	p := httputil.NewSingleHostReverseProxy(target)
	p.Transport = http.DefaultTransport
	return p
}

