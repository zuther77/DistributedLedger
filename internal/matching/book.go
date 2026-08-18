package matching

import (
	"context"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
	"github.com/zuther77/distributed-ledger/internal/redisx"
)

// BookLevel is one resting order on a side of the book.
type BookLevel struct {
	OrderID string `json:"order_id"`
	Price   string `json:"price"`
}

// nested client, use Book.RDB.RDB
type Book struct {
	RDB *redisx.Client
}

// Rest a BUY on the bid side when it cannot match immediately.
func (b * Book) AddBid(ctx context.Context, ticker, orderID, price string) error {
	// Rest a BUY on the bid side when it cannot match immediately.
	return b.zAdd(ctx, redisx.BidsKey(ticker), orderID, price)
}

func (b * Book) AddAsk(ctx context.Context, ticker, orderID, price string) error {
	// Rest a SELL on the ask side when it cannot match immediately.
	return b.zAdd(ctx, redisx.AsksKey(ticker), orderID, price)
}


// define zAdd using redis.ZAddArgs
func (b *Book) zAdd(ctx context.Context, key, orderID, price string) error {
	score , err := strconv.ParseFloat(price, 64)
	if err != nil {
		return fmt.Errorf("parse price %q: %w", price, err)
	}

	return b.RDB.RDB.ZAddArgs(ctx, key, redis.ZAddArgs{
		Members: []redis.Z{ {Score: score, Member: orderID} },
	}).Err()
}


func (b *Book) Remove(ctx context.Context, ticker, orderID string) error {
	// Remove from both; only one side will actually contain it.
	if err := b.RDB.RDB.ZRem(ctx, redisx.BidsKey(ticker), orderID).Err(); err != nil {
		return err
	}
	
	return b.RDB.RDB.ZRem(ctx, redisx.AsksKey(ticker), orderID).Err()

	
}

// ZADD orderbook:AAPL:asks 150 sell-1
// ZADD orderbook:AAPL:asks 149 sell-2
// ZADD orderbook:AAPL:bids 148 buy-1
// ZADD orderbook:AAPL:bids 151 buy-2

// BestAsk → member sell-2, score 149   (lowest ask wins for sellers)
// BestBid → member buy-2,  score 151   (highest bid wins for buyers)
func (b *Book) BestAsk(ctx context.Context, ticker string) (orderID, price string, ok bool, err error) {
	vals, err := b.RDB.RDB.ZRangeArgsWithScores(ctx, redis.ZRangeArgs{
		Key: redisx.AsksKey(ticker),
		Start: 0,
		Stop: 0,
	}).Result()
	if err != nil {
		return "","",false,err
	}

	if len(vals) == 0 {
		// Empty book is normal, not an error.
		return "", "", false, nil
	}

	// Member is the order_id we stored with ZADD (a string).
	orderID, _ = vals[0].Member.(string)
	return orderID, formatPrice(vals[0].Score), true, nil
}


// return highest bid
func (b *Book) BestBid(ctx context.Context, ticker string) (orderID, price string, ok bool, err error) {
	vals , err := b.RDB.RDB.ZRangeArgsWithScores(ctx, redis.ZRangeArgs{
		Key: redisx.BidsKey(ticker),
		Start: 0,
		Stop: 0,
		Rev: true,   // descending → highest bid first
	}).Result()

	if err != nil || len(vals) == 0 {
		return "","",false, err
	}

	if len(vals) == 0 {
		// Empty book is normal, not an error.
		return "", "", false, nil
	}

	// Member is the order_id we stored with ZADD (a string).
	orderID, _ = vals[0].Member.(string)
	return orderID, formatPrice(vals[0].Score), true, nil
}


// IsOnBook reports whether orderID is resting on the book for ticker's side ("BUY" or "SELL").
func (b *Book) IsOnBook(ctx context.Context, ticker, side, orderID string) (bool, error) {
	key := redisx.AsksKey(ticker)
	if side == "BUY" {
		key = redisx.BidsKey(ticker)
	}
	_, err := b.RDB.RDB.ZScore(ctx, key, orderID).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListBids returns up to limit highest bids (best first).
func (b *Book) ListBids(ctx context.Context, ticker string, limit int64) ([]BookLevel, error) {
	if limit <= 0 {
		limit = 50
	}
	vals, err := b.RDB.RDB.ZRangeArgsWithScores(ctx, redis.ZRangeArgs{
		Key:   redisx.BidsKey(ticker),
		Start: 0,
		Stop:  limit - 1,
		Rev:   true,
	}).Result()
	if err != nil {
		return nil, err
	}
	return zToLevels(vals), nil
}

// ListAsks returns up to limit lowest asks (best first).
func (b *Book) ListAsks(ctx context.Context, ticker string, limit int64) ([]BookLevel, error) {
	if limit <= 0 {
		limit = 50
	}
	vals, err := b.RDB.RDB.ZRangeArgsWithScores(ctx, redis.ZRangeArgs{
		Key:   redisx.AsksKey(ticker),
		Start: 0,
		Stop:  limit - 1,
	}).Result()
	if err != nil {
		return nil, err
	}
	return zToLevels(vals), nil
}

func zToLevels(vals []redis.Z) []BookLevel {
	levels := make([]BookLevel, 0, len(vals))
	for _, z := range vals {
		orderID, _ := z.Member.(string)
		levels = append(levels, BookLevel{
			OrderID: orderID,
			Price:   formatPrice(z.Score),
		})
	}
	return levels
}

// helper functions
func formatPrice(score float64) string {
	return strconv.FormatFloat(score, 'f', 2, 64)
}

