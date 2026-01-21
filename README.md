# krono-api (Gin starter)

Minimal starter for a Go API using Gin with a `/health` endpoint.

Run locally:

```bash
# fetch modules
go mod tidy

# run
go run main.go
```

Test:

```bash
curl -sS http://localhost:8080/health
# => {"status":"ok"}
```
