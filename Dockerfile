FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/balancer ./cmd/balancer
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/backend ./cmd/backend

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/balancer /usr/local/bin/balancer
COPY --from=build /out/backend /usr/local/bin/backend
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/balancer"]

