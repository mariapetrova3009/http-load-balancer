package balancer

import "net/url"

type Backend struct {
	Name string
	URL  *url.URL
	// Alive/Requests будут защищены mutex/atomic на этапе реализации.
}

