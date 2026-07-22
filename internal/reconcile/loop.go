package reconcile

import (
	"context"
	"log"
	"time"

	"github.com/zuther77/distributed-ledger/internal/orders"
	"github.com/zuther77/distributed-ledger/internal/redisx"
)

// Loop logic
// every 60 seconds
// 		find PENDING older than 2 min
//		for each: EnqueueOrder(id) again

func Start( orderRepo *orders.Repository, redisClient *redisx.Client) {
	// seperate go routine runs in background
	go func() {
		for {
			// intential wait to not slam postgres at startup
			time.Sleep(60 * time.Second)

			if err := runOnce(context.Background(), orderRepo, redisClient); err != nil {
				log.Printf("reconcile: %v", err)
			}
		}
	}()
}

// one reconcile pass (find stale PENDING → re-XADD).
func runOnce(ctx context.Context, orderRepo *orders.Repository, rediClient *redisx.Client) error {
	staleOrders, err := orderRepo.ListStalePending(ctx, 120 * time.Second)
	if err != nil {
		return err
	}

	for _, order := range staleOrders {
		if err := rediClient.EnqueueOrder(ctx, order.ID); err != nil {
			log.Printf("reconcile enqueue %s: %v", order.ID, err)
			continue
		}
		log.Printf("reconcile: re-queued order %s", order.ID)
	}
	
	return nil
}