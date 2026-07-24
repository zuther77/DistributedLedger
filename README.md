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
│   └── k8s/                    # Minikube manifests (+ prometheus, grafana)
├── loadtest/
│   └── smoke.py                # SELL then BUY demo
│   └── spamAPI.py              # Load test order-api using locust 
├── Dockerfile                  # Multi-stage; SERVICE build-arg
└── README.md
```

---

## Prerequisites

- Docker Desktop (or Docker Engine + Compose v2)
- Go 1.22+ (only if running binaries outside Compose)
- Python 3 + locust (``` pip install locust```) (for `loadtest/smoke.py` and `loadtest/spamApi.py`)
- Optional for Kubernetes path: `kubectl`, `minikube`

---

## Quick start (Docker Compose)

From this directory (`DistributedLedger/`):

```bash
docker compose -f deploy/docker-compose.yml up --build
```

Open Prometheus ```http://localhost:9090``` → Status → Targets — all three should be UP

Open Grafana ```http://localhost:3000```

Import Dashboard from ```deploy/grafana/ledger-dashboard.json```
Grafana: login `admin` / `admin`.


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


### Load test
```
cd loadtest
locust -f loadtest/spamAPI.py --host http://localhost:8080
```
Open ```http://localhost:8089``` — start small (10 users)


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
```

### Port-forward (API + observability)

```bash
kubectl -n ledger port-forward svc/order-api 8080:8080      # smoke / curl / Locust
kubectl -n ledger port-forward svc/prometheus 9090:9090    # http://localhost:9090
kubectl -n ledger port-forward svc/grafana 3000:3000        # http://localhost:3000
```

Add Prometheus datasource URL `http://prometheus:9090` (Service DNS inside the cluster).


Inspect DB:

```bash
kubectl -n ledger exec -it deploy/postgres -- psql -U ledger -d ledger
```


## Cleanup 

```bash
# Compose
docker compose -f deploy/docker-compose.yml down

# Minikube
kubectl delete namespace ledger 2>/dev/null || true
eval $(minikube docker-env -u) 2>/dev/null || true
minikube stop
```

---

