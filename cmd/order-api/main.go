package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zuther77/distributed-ledger/internal/config"
	"github.com/zuther77/distributed-ledger/internal/db"
	"github.com/zuther77/distributed-ledger/internal/orders"
	"github.com/zuther77/distributed-ledger/internal/redisx"
)

func main() {
    // loading config
    cfg := config.Load()

    // Timeout so a hung DB does not freeze startup forever.
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // connect to postgres
    pool , err := db.Connect(ctx, cfg.DatabaseUrl)
    if err != nil {
        log.Fatalf("postgres: %v", err)
    }
    defer pool.Close()

    // Connect to Redis
    rdb , err := redisx.Connect(cfg.RedisAddr)
    if err != nil {
        log.Fatalf("Redis: %v", err)
    }
    defer rdb.Close()

    // pass handlers 
    h := &orders.Handlers{
        Repo: &orders.Repository{DB: pool},
        Redis: rdb,
    }
    r := gin.Default()

    v1 := r.Group("/api/v1")
    v1.POST("/orders", h.CreateOrder)
    v1.GET("/orders/:id", h.GetOrder)
        
    log.Printf("order-api listening on %2=s", cfg.HTTPAddr)
    if err := r.Run(cfg.HTTPAddr); err != nil {
        log.Fatalf("http server: %v", err )
    }

    // release port 
    

}