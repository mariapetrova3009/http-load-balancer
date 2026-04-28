package balancer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type HealthChecker struct {
	client   *http.Client
	interval time.Duration
	path     string
}

type HealthCheckerOption func(*HealthChecker)

func WithHealthPath(path string) HealthCheckerOption {
	return func(h *HealthChecker) { h.path = path }
}

func NewHealthChecker(interval, timeout time.Duration, opts ...HealthCheckerOption) *HealthChecker {
	h := &HealthChecker{
		client: &http.Client{
			Timeout: timeout,
		},
		interval: interval,
		path:     "/health",
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *HealthChecker) Run(ctx context.Context, backends []*Backend) {
	t := time.NewTicker(h.interval)
	defer t.Stop()

	// Run an initial probe so /stats is useful right after startup.
	h.checkAll(ctx, backends)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.checkAll(ctx, backends)
		}
	}
}

func (h *HealthChecker) checkAll(ctx context.Context, backends []*Backend) {
	for _, b := range backends {
		alive, err := h.checkOne(ctx, b.URL)
		if err != nil {
			b.SetAlive(false, err.Error())
			continue
		}
		b.SetAlive(alive, "")
	}
}

func (h *HealthChecker) checkOne(ctx context.Context, base *url.URL) (bool, error) {
	u := *base
	u.Path = h.path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return false, err
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, errors.New("health endpoint not found (404)")
	}
	return false, fmt.Errorf("health status %d", resp.StatusCode)
}

