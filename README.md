# Auth Service

This repository contains a sample authentication microservice in Go. It exposes REST endpoints and a gRPC API implementing sign up, login, token refresh and logout flows.

## Running

```
go run ./cmd/authsvc
```

By default the service expects Postgres on `localhost:5432` and Redis on `localhost:6379`.
