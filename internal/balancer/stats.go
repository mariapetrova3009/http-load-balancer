package balancer

import "time"

type Stats struct {
	// TotalRequests counts proxied requests (not /health or /stats).
	TotalRequests uint64         `json:"total_requests"`
	Backends      []BackendStats `json:"backends"`
}

type BackendStats struct {
	Name            string    `json:"name"`
	URL             string    `json:"url"`
	Alive           bool      `json:"alive"`
	ActiveRequests  int       `json:"active_requests"`
	TotalRequests   uint64    `json:"total_requests"`
	LastHealthCheck time.Time `json:"last_health_check,omitempty"`
	LastHealthError string    `json:"last_health_error,omitempty"`
}

