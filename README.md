# Go HTTP Load Balancer

Учебный pet-проект на Go: HTTP reverse-proxy балансировщик нагрузки с round-robin, active health checks backend-сервисов, `/stats`, логированием, таймаутами и graceful shutdown.

## Структура проекта

```
.
├── cmd/
│   ├── balancer/
│   │   └── main.go
│   └── backend/
│       └── main.go
├── internal/
│   ├── balancer/
│   │   ├── backend.go
│   │   ├── health_checker.go
│   │   ├── load_balancer.go
│   │   └── stats.go
│   ├── config/
│   │   └── config.go
│   └── proxy/
│       └── proxy.go
├── Dockerfile
├── docker-compose.yml
└── go.mod
```

## Быстрый старт (без Docker)

Запуск backend-ов:

```bash
go run ./cmd/backend --name backend-1 --port 9001
go run ./cmd/backend --name backend-2 --port 9002
go run ./cmd/backend --name backend-3 --port 9003
```

Запуск балансировщика:

```bash
go run ./cmd/balancer
```

Проверка:

```bash
curl -s http://localhost:8080/health
curl -s http://localhost:8080/stats
curl -s http://localhost:8080/api/test
```

