package config

import "time"

type Config struct {
	Server      ServerConfig
	HealthCheck HealthCheckConfig
	Backends    []BackendConfig
}

type ServerConfig struct {
	Port int
}

type HealthCheckConfig struct {
	Interval time.Duration
	Timeout  time.Duration
}

type BackendConfig struct {
	Name string
	URL  string
}

