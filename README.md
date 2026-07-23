# Distributed Ledger

High-throughput order matching engine in Go with an ACID-compliant PostgreSQL ledger and Redis Streams for durable, fault-tolerant handoff between services. Microservices run via Docker Compose locally and can be deployed to Kubernetes (Minikube).

This is a **learning / portfolio system** — concurrency, distributed state, and container orchestration — not a production exchange.

---

## System architecture

```text
                    ┌─────────────────┐
   Client / smoke   │   order-api     │
   POST /orders ───►│  (HTTP :8080)   │
                    └────────┬────────┘
                             │
              1. INSERT PENDING (Postgres)
              2. XADD order_stream (Redis)
                             │
                             ▼
                    ┌─────────────────┐
                    │ matching-engine │  replicas: 1
                    │  (consumer grp) │
                    └────────┬────────┘
                             │
         Resting orders → Redis ZSET order book
         Match → XADD settlement_stream
                             │
                             ▼
                    ┌─────────────────┐
                    │settlement-worker│
                    │  (consumer grp) │
                    └────────┬────────┘
                             │
         Atomic Postgres txn (balances + FILLED + trade)
         then XACK settlement_stream
                             │
                             ▼
                    ┌──────────────┐     ┌─────────────┐
                    │  PostgreSQL  │     │    Redis    │
                    │ users/orders │     │ streams +   │
                    │ /trades      │     │ ZSET book   │
                    └──────────────┘     └─────────────┘
```

### Request lifecycle

1. **order-api** validates the request, writes the order as `PENDING` in Postgres, and `XADD`s the order ID to `order_stream`.
2. **matching-engine** reads via consumer group `matchers`, updates the Redis ZSET book (`orderbook:{ticker}:bids|asks`), and on a match publishes to `settlement_stream`.
3. **settlement-worker** reads via consumer group `settlers`, settles balances and marks orders `FILLED` in one transaction, then `XACK`s.
4. A **reconcile** loop re-injects stuck `PENDING` orders if they fell out of the stream/book path.

### Why these pieces

| Piece | Role |
|---|---|
| Postgres | Source of truth for money and order status (ACID) |
| Redis Streams | Durable queue; unacked messages can be `XCLAIM`ed after a crash |
| Redis ZSET | Live order book (fast; rebuilt/reconciled on drift) |
| Consumer groups | At-least-once delivery without Pub/Sub fire-and-forget |
| Idempotent SQL | `UPDATE … WHERE status = 'PENDING'` so double-settle is safe |

---

## Repository layout

```text
DistributedLedger/
├── cmd/
│   ├── order-api/              # HTTP API
│   ├── matching-engine/        # Stream consumer + book matcher
│   └── settlement-worker/      # Settles trades in Postgres
├── internal/
│   ├── config/ db/ redisx/
│   ├── orders/ matching/ settlement/ reconcile/
├── migrations/
│   └── 001_init.sql            # Schema + demo users
├── deploy/
│   ├── docker-compose.yml      # Full local stack
│   ├── docker-compose.dev.yml  # Postgres + Redis only
│   └── k8s/                    # Minikube manifests
├── loadtest/
│   └── smoke.py                # SELL then BUY demo
├── Dockerfile                  # Multi-stage; SERVICE build-arg
└── README.md
```

---

## Prerequisites

- Docker Desktop (or Docker Engine + Compose v2)
- Go 1.22+ (only if running binaries outside Compose)
- Python 3 (for `loadtest/smoke.py`)
- Optional for Kubernetes path: `kubectl`, `minikube`

---

## Quick start (Docker Compose)

From this directory (`DistributedLedger/`):

```bash
docker compose -f deploy/docker-compose.yml up --build
```

Compose paths are relative to `deploy/`. Build context is the repo root (`context: ..`) so `go.mod` / `cmd/` are visible to the Dockerfile.

On first empty Postgres volume, `migrations/001_init.sql` creates tables and seeds two demo users:

| Role | UUID | Starting balance |
|---|---|---|
| Buyer | `11111111-1111-1111-1111-111111111111` | 10000 |
| Seller | `22222222-2222-2222-2222-222222222222` | 10000 |

### Smoke test

```bash
python3 loadtest/smoke.py
```

Or manually:

```bash
# SELL first (rest liquidity), then BUY
curl -s -X POST http://localhost:8080/api/v1/orders \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"22222222-2222-2222-2222-222222222222","ticker":"APPL","side":"SELL","qty":"10","price":"150.00"}'

curl -s -X POST http://localhost:8080/api/v1/orders \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"11111111-1111-1111-1111-111111111111","ticker":"APPL","side":"BUY","qty":"10","price":"150.00"}'
```

### Inspect ledger

```bash
docker compose -f deploy/docker-compose.yml exec postgres \
  psql -U ledger -d ledger -c "SELECT * FROM trades; SELECT id, balance FROM users;"
```

### Logs

```bash
docker compose -f deploy/docker-compose.yml logs matching-engine --tail=50
docker compose -f deploy/docker-compose.yml logs settlement-worker --tail=50
```

### Reset local data

Init SQL runs only on a **new** Postgres volume. To wipe and re-seed:

```bash
docker compose -f deploy/docker-compose.yml down -v
docker compose -f deploy/docker-compose.yml up --build
```

---

## Kubernetes (Minikube)

```bash
minikube start
eval $(minikube docker-env)

docker build -t ledger-order-api:dev --build-arg SERVICE=order-api .
docker build -t ledger-matching-engine:dev --build-arg SERVICE=matching-engine .
docker build -t ledger-settlement-worker:dev --build-arg SERVICE=settlement-worker .

kubectl apply -f deploy/k8s/

# Schema + seed (K8s Postgres has no Compose init mount)
kubectl -n ledger exec -i deploy/postgres -- psql -U ledger -d ledger < migrations/001_init.sql

kubectl -n ledger port-forward svc/order-api 8080:8080
# then run smoke.py or curl as above
```

Notes:

- Only **order-api** exposes a Service (HTTP). Matcher and settlement are background workers.
- Keep **matching-engine** at `replicas: 1` for this design (shared Redis book, no ticker sharding yet).
- Images must be built into Minikube’s Docker (`eval $(minikube docker-env)`); Deployments use `imagePullPolicy: IfNotPresent`.

Inspect DB:

```bash
kubectl -n ledger exec -it deploy/postgres -- psql -U ledger -d ledger
```



## Cleanup (save CPU / RAM)

```bash
# Compose
docker compose -f deploy/docker-compose.yml down

# Minikube
kubectl delete namespace ledger 2>/dev/null || true
eval $(minikube docker-env -u) 2>/dev/null || true
minikube stop
```

---

