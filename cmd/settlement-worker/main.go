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
	"github.com/zuther77/distributed-ledger/internal/metrics"
	"github.com/zuther77/distributed-ledger/internal/redisx"
	"github.com/zuther77/distributed-ledger/internal/settlement"
)

func main() {
	// Metrics HTTP for Prometheus (scrapes :2112/metrics on the Compose network).
	go func() {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		log.Println("metrics listening on :2112")
		_ = http.ListenAndServe(":2112", metricsMux)
	}()

	config := config.Load()
	ctx := context.Background()


	// ddatabase connect
	pool , err := db.Connect(ctx, config.DatabaseUrl)
	if err != nil {
		log.Fatalf("Postgres connect err : %v", err)
	}
	defer pool.Close()

	// redis connect
	rdb, err := redisx.Connect(config.RedisAddr)
	if err != nil {
		log.Fatalf("Redis connect err : %v", err)
	}
	defer rdb.Close()
	

	// Consumer group must be on settlement_stream (where matches are written),
	// NOT order_stream. Otherwise ReadGroup never sees match events.
	if err := rdb.EnsureGroup(ctx, redisx.StreamSettlement, redisx.GroupSettlers); err != nil {
		log.Fatalf("ensuregroup : %v", err)
	}

	settlementWorker := &settlement.Worker{DB: pool}
	consumer := hostnameOr("settler-1")
	log.Printf("settlement-worker as %s", consumer)

	for {
		staleMsgs , _ := rdb.ClaimStale(ctx, redisx.StreamSettlement, redisx.GroupSettlers, consumer, 60 * time.Second)

		for _, streamMsg := range staleMsgs {
			handleSettlementMsg(ctx, settlementWorker, rdb, streamMsg.ID, streamMsg.Values)
		}
		
		streamMessages, err := rdb.ReadGroup(ctx, redisx.StreamSettlement, redisx.GroupSettlers, consumer, 10)
		if err != nil {
			log.Printf("Readgroup error : %v", err)
			time.Sleep(time.Second)
			continue
		}

		for _, streamMsg := range streamMessages {
			handleSettlementMsg(ctx, settlementWorker, rdb, streamMsg.ID, streamMsg.Values)
		}
 	}
}


func handleSettlementMsg(ctx context.Context, settlementWorker *settlement.Worker, rdb *redisx.Client, messageID string, msgFields map[string]interface{}) {
	matchEvent, err := settlement.ParseEvent(msgFields)
	if err != nil {
		// Bad payload cannot succeed later — ack to unblock the group.
		log.Printf("bad event %s: %v — acking", messageID,err)
		_ = rdb.Ack(ctx, redisx.StreamSettlement, redisx.GroupSettlers, messageID)
		return
	}

	if err := settlementWorker.Settle(ctx, matchEvent); err != nil {
		metrics.SettlementErrors.Inc()
		log.Printf("settle %s: %v (will retry)", matchEvent.TradeID, err)
		return // do NOT ack
	}

	_ = rdb.Ack(ctx, redisx.StreamSettlement, redisx.GroupSettlers, messageID)
}

func hostnameOr(fallback string) string {
	if hostName, err := os.Hostname(); err == nil && hostName != "" {
		return hostName
	}
	return fallback
}