package balancer

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync/atomic"

	"go-http-load-balancer/internal/proxy"
)

var ErrNoBackends = errors.New("no backends available")

// LoadBalancer хранит backend-ы и раздаёт запросы по round-robin среди живых.
type LoadBalancer struct {
	backends []*Backend
	idx      uint64

	totalRequests atomic.Uint64
}

type Option func(*LoadBalancer)

func WithHealthChecks(hc *HealthChecker) Option {
	return func(lb *LoadBalancer) {
		// HealthChecker стартуем из cmd, здесь опция для расширения/совместимости.
		_ = hc
	}
}

func New(backends []*url.URL, opts ...Option) *LoadBalancer {
	bs := make([]*Backend, 0, len(backends))
	for i, u := range backends {
		p := proxy.NewReverseProxy(u)
		bs = append(bs, NewBackend(fmt.Sprintf("backend-%d", i+1), u, p))
	}

	lb := &LoadBalancer{backends: bs}
	for _, opt := range opts {
		opt(lb)
	}
	return lb
}

func (lb *LoadBalancer) Backends() []*Backend { return lb.backends }

func (lb *LoadBalancer) next() (*Backend, error) {
	if len(lb.backends) == 0 {
		return nil, ErrNoBackends
	}

	// Ищем живой backend, начиная с round-robin индекса.
	start := atomic.AddUint64(&lb.idx, 1) - 1
	n := len(lb.backends)
	for step := 0; step < n; step++ {
		b := lb.backends[int((start+uint64(step))%uint64(n))]
		if b.IsAlive() {
			return b, nil
		}
	}
	return nil, ErrNoBackends
}

// ServeHTTP проксирует запрос на следующий backend.
func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, err := lb.next()
	if err != nil {
		http.Error(w, "no backends available", http.StatusServiceUnavailable)
		return
	}

	lb.totalRequests.Add(1)
	b.IncTotal()
	b.IncActive()
	defer b.DecActive()

	b.Proxy().ServeHTTP(w, r)
}

func (lb *LoadBalancer) Stats() Stats {
	out := Stats{
		TotalRequests: lb.totalRequests.Load(),
		Backends:      make([]BackendStats, 0, len(lb.backends)),
	}
	for _, b := range lb.backends {
		out.Backends = append(out.Backends, BackendStats{
			Name:            b.Name,
			URL:             b.URL.String(),
			Alive:           b.IsAlive(),
			ActiveRequests:  b.Active(),
			TotalRequests:   b.Total(),
			LastHealthCheck: b.LastHealthCheck(),
			LastHealthError: b.LastHealthError(),
		})
	}
	return out
}

