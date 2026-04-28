package balancer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

	maxRetries int
	maxBody    int64
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

	lb := &LoadBalancer{backends: bs, maxRetries: 1, maxBody: 1 << 20}
	for _, opt := range opts {
		opt(lb)
	}
	return lb
}

func (lb *LoadBalancer) Backends() []*Backend { return lb.backends }

func (lb *LoadBalancer) SetMaxRetries(n int) {
	if n < 0 {
		n = 0
	}
	lb.maxRetries = n
}

func (lb *LoadBalancer) SetMaxBodyBytes(n int64) {
	if n <= 0 {
		n = 1 << 20
	}
	lb.maxBody = n
}

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
	canRetry := lb.maxRetries > 0 && (r.Method == http.MethodGet || r.Method == http.MethodHead)
	attempts := 1
	if canRetry {
		attempts = lb.maxRetries + 1
	}
	var lastErr error

	// Чтобы можно было безопасно ретраить, читаем body один раз и переиспользуем (с лимитом).
	var bodyBuf []byte
	if canRetry && r.Body != nil {
		buf, tooLarge, _ := readBodyWithLimit(r.Body, lb.maxBody)
		if tooLarge {
			http.Error(w, "request entity too large", http.StatusRequestEntityTooLarge)
			return
		}
		bodyBuf = buf
	}

	for i := 0; i < attempts; i++ {
		b, err := lb.next()
		if err != nil {
			http.Error(w, "no backends available", http.StatusServiceUnavailable)
			return
		}

		var req *http.Request
		if canRetry {
			req = r.Clone(r.Context())
			if bodyBuf != nil {
				req.Body = io.NopCloser(bytes.NewReader(bodyBuf))
				req.ContentLength = int64(len(bodyBuf))
			}
		} else {
			// Без ретраев не трогаем исходный body.
			req = r
		}

		lb.totalRequests.Add(1)
		b.IncTotal()
		b.IncActive()

		rec := httptest.NewRecorder()
		var proxyErr error
		p := b.Proxy()
		prev := p.ErrorHandler
		p.ErrorHandler = func(rw http.ResponseWriter, rr *http.Request, e error) {
			proxyErr = e
			// Не пишем ответ в rw, чтобы можно было ретраить.
		}
		p.ServeHTTP(rec, req)
		p.ErrorHandler = prev

		b.DecActive()

		if proxyErr != nil {
			lastErr = proxyErr
			b.SetAlive(false, proxyErr.Error())
			continue
		}

		// Успех: копируем буферизованный ответ в клиента.
		for k, vv := range rec.Header() {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
		return
	}

	if lastErr != nil {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
}

func readBodyWithLimit(rc io.ReadCloser, max int64) ([]byte, bool, error) {
	defer rc.Close()
	// max+1, чтобы понять что тело больше лимита.
	b, err := io.ReadAll(io.LimitReader(rc, max+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(b)) > max {
		return nil, true, nil
	}
	return b, false, nil
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

