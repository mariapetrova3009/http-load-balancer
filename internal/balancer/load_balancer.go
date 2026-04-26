package balancer

import (
	"errors"
	"net/http"
	"net/url"
	"sync/atomic"

	"go-http-load-balancer/internal/proxy"
)

var ErrNoBackends = errors.New("no backends available")

// LoadBalancer для MVP держит список backend URL и делает round-robin.
type LoadBalancer struct {
	backends []*url.URL
	idx      uint64
}

func New(backends []*url.URL) *LoadBalancer {
	return &LoadBalancer{backends: backends}
}

func (lb *LoadBalancer) next() (*url.URL, error) {
	if len(lb.backends) == 0 {
		return nil, ErrNoBackends
	}
	i := atomic.AddUint64(&lb.idx, 1)
	return lb.backends[int(i-1)%len(lb.backends)], nil
}

// ServeHTTP проксирует запрос на следующий backend.
// Health checks и исключение dead backend-ов добавим в следующей итерации.
func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, err := lb.next()
	if err != nil {
		http.Error(w, "no backends available", http.StatusServiceUnavailable)
		return
	}

	p := proxy.NewReverseProxy(b)
	p.ServeHTTP(w, r)
}

