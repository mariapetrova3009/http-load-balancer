package config

import (
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"
)

type Config struct {
	Server      ServerConfig
	HealthCheck HealthCheckConfig
	Retry       RetryConfig
	Backends    []BackendConfig
}

type ServerConfig struct {
	Port int
}

type HealthCheckConfig struct {
	Interval time.Duration
	Timeout  time.Duration
}

type RetryConfig struct {
	MaxRetries int
}

type BackendConfig struct {
	Name string
	URL  string
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Port: 8080,
		},
		HealthCheck: HealthCheckConfig{
			Interval: 2 * time.Second,
			Timeout:  800 * time.Millisecond,
		},
		Retry: RetryConfig{
			MaxRetries: 1,
		},
		Backends: []BackendConfig{
			{Name: "backend-1", URL: "http://localhost:9001"},
			{Name: "backend-2", URL: "http://localhost:9002"},
			{Name: "backend-3", URL: "http://localhost:9003"},
		},
	}
}

type FlagArgs struct {
	Port       int
	Backends   string
	HCInterval time.Duration
	HCTimeout  time.Duration
	Retries    int
}

func FromFlags() (Config, error) {
	def := Default()

	var a FlagArgs
	flag.IntVar(&a.Port, "port", def.Server.Port, "balancer listen port")
	flag.StringVar(&a.Backends, "backends", joinBackendURLs(def.Backends), "comma-separated backend base URLs (e.g. http://localhost:9001,http://localhost:9002)")
	flag.DurationVar(&a.HCInterval, "hc-interval", def.HealthCheck.Interval, "health check interval")
	flag.DurationVar(&a.HCTimeout, "hc-timeout", def.HealthCheck.Timeout, "health check timeout")
	flag.IntVar(&a.Retries, "retries", def.Retry.MaxRetries, "max retries on proxy error (0 = disable)")
	flag.Parse()

	cfg := def
	cfg.Server.Port = a.Port
	cfg.HealthCheck.Interval = a.HCInterval
	cfg.HealthCheck.Timeout = a.HCTimeout
	if a.Retries < 0 {
		return Config{}, errors.New("retries must be >= 0")
	}
	cfg.Retry.MaxRetries = a.Retries

	bes, err := parseBackends(a.Backends)
	if err != nil {
		return Config{}, err
	}
	cfg.Backends = bes

	return cfg, nil
}

func joinBackendURLs(bes []BackendConfig) string {
	out := make([]string, 0, len(bes))
	for _, b := range bes {
		out = append(out, b.URL)
	}
	return strings.Join(out, ",")
}

func parseBackends(raw string) ([]BackendConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("backends is empty")
	}
	parts := strings.Split(raw, ",")
	out := make([]BackendConfig, 0, len(parts))
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, BackendConfig{
			Name: fmt.Sprintf("backend-%d", i+1),
			URL:  p,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("no valid backends parsed")
	}
	return out, nil
}
