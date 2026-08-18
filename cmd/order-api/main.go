package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zuther77/distributed-ledger/internal/config"
	"github.com/zuther77/distributed-ledger/internal/db"
	"github.com/zuther77/distributed-ledger/internal/orderbook"
	"github.com/zuther77/distributed-ledger/internal/orders"
	"github.com/zuther77/distributed-ledger/internal/redisx"
)

func main() {
    // loading config
    config := config.Load()

    // Timeout so a hung DB does not freeze startup forever.
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // connect to postgres
    pool , err := db.Connect(ctx, config.DatabaseUrl)
    if err != nil {
        log.Fatalf("postgres: %v", err)
    }
    defer pool.Close()

    // Connect to Redis
    rdb , err := redisx.Connect(config.RedisAddr)
    if err != nil {
        log.Fatalf("Redis: %v", err)
    }
    defer rdb.Close()

    // pass handlers 
    handlers := &orders.Handlers{
        Repo: &orders.Repository{DB: pool},
        Redis: rdb,
    }
    bookHandler := &orderbook.Handler{Redis: rdb}
    ginEngine := gin.Default()

    // group created so apis stay under /api/v1
    v1 := ginEngine.Group("/api/v1")
    v1.POST("/orders", handlers.CreateOrder)
    v1.GET("/orders/:id", handlers.GetOrder)
    v1.GET("/orderbook/:ticker", bookHandler.Get)

    // metrics endpoint. scrappers expect this path at the server root
    ginEngine.GET("/metrics", gin.WrapH(promhttp.Handler()))

    log.Printf("order-api listening on %s", config.HTTPAddr)
    if err := ginEngine.Run(config.HTTPAddr); err != nil {
        log.Fatalf("http server: %v", err )
    }

    // release port 
    

}