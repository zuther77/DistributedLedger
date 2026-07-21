package redisx

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	StreamOrders 		= "order_stream"
	StreamSettlement 	= "settlement_stream"
	GroupMatchers		= "matchers"
	GroupSettlers		= "settlers"
)


type Client struct {
	RDB *redis.Client
}


// Connect to redis and Ping. Log errs if ping fails
func Connect(addr string) (*Client, error) {
	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// go-redis Ping returns a StatusCmd object. we only use Err()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("Redis ping: %w", err)
	}
	
    fmt.Println("Connected to Redis Successfull")

	return &Client{RDB: rdb}, nil
}

func (c * Client) Close() error {
	if c == nil || c.RDB == nil {
		return nil
	}
	return c.RDB.Close()
}



// Push a ticket on stream 
func ( c *Client ) EnqueueOrder(ctx context.Context, orderID string) error {

	// Reddis command - XADD orders * order_id 12345
	// using ID: "*" means redis will auto-generate an ID for us
    _, err := c.RDB.XAdd(ctx, &redis.XAddArgs{
        Stream: StreamOrders,
		ID: "*",
		Values: map[string]interface{}{
			"order_id" : orderID,
		},
    }).Result()

	if err != nil {
		return fmt.Errorf("XADD %s: %w", StreamOrders, err)
	}

	return nil
}


// create a consumer group once
func (c *Client) EnsureGroup(ctx context.Context, stream, group string) error {
	// "0" = read from the beginning 
	// "$" = only messages created after the group exists 
	err := c.RDB.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	// "BUSYGROUP" means it already exists — that is OK (idempotent startup).
	if err != nil && !redis.HasErrorPrefix(err , "BUSYGROUP") {
		return err
	}

	return nil
}

// read new messages for a consumer 
func (c *Client) ReadGroup(ctx context.Context, stream, group, consumer string, count int64) ([]redis.XMessage, error) {

	streams, err := c.RDB.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: 		group,
		Consumer: 	consumer,
		Streams: 	[]string{stream, ">"},   	// ">" = only never-delivered messages
		Count: 		count,
		Block: 		2 * time.Second,             // wait a bit instead of busy-looping

	}).Result()
	if err == redis.Nil || len(streams) ==0 {
		return nil, nil
	}
	if err != nil{
		return nil, err
	}
	
	return streams[0].Messages, nil
}

func (c *Client) Ack(ctx context.Context, stream, group, messageID string) error {
	return c.RDB.XAck(ctx, stream, group, messageID).Err()
}

// take message stuck with dead consumer idle >= minIdle
func (c *Client) ClaimStale(ctx context.Context, stream, group, consumer string, minIdle time.Duration) ([]redis.XMessage, error) {

	pending, err := c.RDB.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: 	stream,
		Group: 		group,
		Start: 		"-",
		End: 		"+",
		Count: 		10,
	}).Result()

	if err != nil {
		return nil, err
	}

	var ids []string
	for _,p := range pending {
		if p.Idle >= minIdle {
			ids = append(ids, p.ID)
		}
	}

	if len(ids) == 0 {
		return nil,  nil
	}

	return c.RDB.XClaim(ctx, &redis.XClaimArgs{
		Stream: stream,
		Group: group,
		Consumer: consumer,
		MinIdle: minIdle,
		Messages: ids,
	}).Result()

}

// Book key helpers — one ZSET per ticker per side.
// BidsKey("AAPL") → "orderbook:AAPL:bids"
// AsksKey("AAPL") → "orderbook:AAPL:asks"
func BidsKey(ticker string) string { return "orderbook:" + ticker + ":bids" }
func AsksKey(ticker string) string { return "orderbook:" + ticker + ":asks" }

