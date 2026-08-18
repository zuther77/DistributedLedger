package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zuther77/distributed-ledger/internal/config"
	"github.com/zuther77/distributed-ledger/internal/db"
	"github.com/zuther77/distributed-ledger/internal/matching"
	"github.com/zuther77/distributed-ledger/internal/metrics"
	"github.com/zuther77/distributed-ledger/internal/orders"
	"github.com/zuther77/distributed-ledger/internal/reconcile"
	"github.com/zuther77/distributed-ledger/internal/redisx"
)

/* loop logic:
 forever loop:
   1) ClaimStale ( steal stuck tickers older than 60s)
   2) process each claimed message
   3) ReadGroup ( new tickets)
   4) process each message:
			call HandleOrderByID
			if err -> retry later
			if ok -> ACK
*/ 

func main() {
	// go routine for metrics start
	go func ()  {
		metrticsMux := http.NewServeMux()
		metrticsMux.Handle("/metrics", promhttp.Handler())
		log.Printf("Metrics listening on port:2112")
		_ = http.ListenAndServe(":2112", metrticsMux)
	} ()

	config := config.Load()
	ctx := context.Background()

	// db conect
	pool, err := db.Connect(ctx, config.DatabaseUrl)
	if err != nil {
		log.Fatalf("Postgres error: %v", err)
	}
	defer pool.Close()

	// redis connect
	rdb, err := redisx.Connect(config.RedisAddr)
	if err != nil {
		log.Fatalf("Redis error: %v", err)
	}
	defer rdb.Close()


	// Create matchers group on order_stream
	if err := rdb.EnsureGroup(ctx, redisx.StreamOrders, redisx.GroupMatchers); err != nil {
		log.Fatalf("Redis consumer group error: %v", err)
	}

	repo := &orders.Repository{DB: pool}
	engine := &matching.Engine{
		Redis: rdb,
		Repo: repo,
		Book: &matching.Book{RDB: rdb},
	}

	reconcile.Start(repo, rdb)
	consumer := hostnameOr("matcher-1")
	log.Printf("matching-engine started as %s", consumer)

	for {
		// dont't let stuck tickets rot forever
		stale, err := rdb.ClaimStale(ctx, redisx.StreamOrders, redisx.GroupMatchers, consumer, 60 * time.Second)
		if err != nil {
			log.Printf("error from ClaimStale: %v", err)
			time.Sleep(time.Second)
			continue
		}

		for _ , msg := range stale {
			processOrderMsg(ctx, engine, rdb, msg.ID, msg.Values)
		}

		// read messages from group
		messages, err := rdb.ReadGroup(ctx, redisx.StreamOrders, redisx.GroupMatchers, consumer, 10)
		if err != nil {
			log.Printf("read group error : %v", err)
			time.Sleep(time.Second)
			continue
		}

		for _ , msg := range messages {
			processOrderMsg(ctx, engine, rdb, msg.ID, msg.Values)
		}
	}
}

func streamFieldString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return ""
	}
}

func processOrderMsg(ctx context.Context, engine *matching.Engine, rdb *redisx.Client, id string, values map[string]interface{}) {
	orderID := streamFieldString(values["order_id"])
	if orderID == "" {
		log.Printf("message %s mission order_id - acknowledging", id)
		_ = rdb.Ack(ctx, redisx.StreamOrders, redisx.GroupMatchers, id)
		return
	}
	startedAt := time.Now()
	if err := engine.HandleOrderByID(ctx, orderID); err != nil {
		// Leave unacked → stays pending → ClaimStale/retry can redo it.
		log.Printf("Handle %s: %v (will retry)", orderID, err)
		return
	}
	metrics.MatchingLatency.Observe(float64(time.Since(startedAt).Seconds()))

	if err := rdb.Ack(ctx, redisx.StreamOrders, redisx.GroupMatchers, id); err != nil {
		log.Printf("acknowledged failed ID %s: %v", id, err)
	}
}


func hostnameOr(fallback string) string {
	if hostName, err := os.Hostname(); err == nil && hostName != "" {
		return hostName
	}
	return fallback
}