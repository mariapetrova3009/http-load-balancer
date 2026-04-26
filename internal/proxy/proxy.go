package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// NewReverseProxy создаёт reverse proxy на конкретный backend.
// Для MVP нам достаточно стандартного ReverseProxy.
func NewReverseProxy(target *url.URL) *httputil.ReverseProxy {
	p := httputil.NewSingleHostReverseProxy(target)

	// Чтобы не зависеть от глобального http.DefaultTransport.
	p.Transport = http.DefaultTransport

	return p
}

