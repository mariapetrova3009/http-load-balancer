package balancer

// HealthChecker будет периодически дергать /health каждого backend-а
// и помечать их alive/dead.
type HealthChecker struct{}

