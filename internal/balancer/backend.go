package balancer

import (
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"
)

type Backend struct {
	Name string
	URL  *url.URL

	alive         atomic.Bool
	totalRequests atomic.Uint64
	active        atomic.Int64

	lastHealthCheckUnix atomic.Int64
	lastHealthErr       atomic.Value

	proxy *httputil.ReverseProxy
}

func NewBackend(name string, u *url.URL, p *httputil.ReverseProxy) *Backend {
	b := &Backend{
		Name:  name,
		URL:   u,
		proxy: p,
	}
	b.alive.Store(true)
	b.lastHealthErr.Store("")
	return b
}

func (b *Backend) IsAlive() bool { return b.alive.Load() }

func (b *Backend) SetAlive(alive bool, errMsg string) {
	b.alive.Store(alive)
	b.lastHealthCheckUnix.Store(time.Now().Unix())
	if errMsg == "" {
		b.lastHealthErr.Store("")
	} else {
		b.lastHealthErr.Store(errMsg)
	}
}

func (b *Backend) IncActive()  { b.active.Add(1) }
func (b *Backend) DecActive()  { b.active.Add(-1) }
func (b *Backend) IncTotal()   { b.totalRequests.Add(1) }
func (b *Backend) Active() int { return int(b.active.Load()) }
func (b *Backend) Total() uint64 {
	return b.totalRequests.Load()
}

func (b *Backend) LastHealthCheck() time.Time {
	sec := b.lastHealthCheckUnix.Load()
	if sec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, 0)
}

func (b *Backend) LastHealthError() string {
	v := b.lastHealthErr.Load()
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (b *Backend) Proxy() *httputil.ReverseProxy { return b.proxy }

