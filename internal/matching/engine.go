package matching

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/zuther77/distributed-ledger/internal/orders"
	"github.com/zuther77/distributed-ledger/internal/redisx"
)

type MatchEvent struct {
	TradeID 	string
	BuyOrderID	string
	SellOrderID string
	Ticker		string
	Price 		string
	Qty 		string
}

type Engine struct {
	Redis *redisx.Client
	Repo *orders.Repository
	Book *Book
}


// HandleOrderByID(orderID)
//   load order from Postgres
//   if not PENDING → return (duplicate/stale ticket)
//   if BUY  → handleBuy
//   if SELL → handleSell
func (e *Engine) HandleOrderByID(ctx context.Context, orderID string) error {
	order, err := e.Repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}

	if order.Status != orders.StatusPending {
		return  nil
	}

	switch order.Side {
	case orders.SideBuy:
		return e.handleBuy(ctx, order)
	case orders.SideSell:
		return e.handleSell(ctx, order)
	default:
		return fmt.Errorf("Unknown side %q", order.Side)
	}
}

//Incoming BUY matches if there is an ask with price ≤ buy price.
// add bid is askprice < buy price or if no bid exist
func (e * Engine) handleBuy(ctx context.Context, buy orders.Order) error {
	askID, askPrice, ok, err := e.Book.BestAsk(ctx, buy.Ticker)

	if err != nil {
		return err
	}

	if !ok {
		return e.Book.AddBid(ctx, buy.Ticker, buy.ID, buy.Price)
	}

	buyP, _ := strconv.ParseFloat(buy.Price, 64)
	askP, _ := strconv.ParseFloat(askPrice, 64)
	if buyP < askP {
		// Willing to pay less than best ask → cannot trade → rest as bid.
		return e.Book.AddBid(ctx, buy.Ticker, buy.ID, buy.Price)
	}

	// buy qty must be equal to sell qty, against best ask 
	sell, err := e.Repo.GetOrderByID(ctx, askID)
	if err != nil {
		return err
	}

	if sell.Status != orders.StatusPending || sell.Quantity != buy.Quantity {
		return e.Book.AddBid(ctx, buy.Ticker, buy.ID, buy.Price)

	}
	
	return e.emitMatch(ctx, buy, sell, askPrice)

}

// Incoming SELL matches if there is a bid with price ≥ sell price.
func (e * Engine) handleSell(ctx context.Context, sell orders.Order) error {
	bidID, bidPrice, ok, err := e.Book.BestBid(ctx, sell.Ticker)
	if err != nil {
		return err
	}

	if !ok {
		return e.Book.AddAsk(ctx, sell.Ticker, sell.ID, sell.Price)
	}

	sellP , _ := strconv.ParseFloat(sell.Price, 64)
	bidP , _ := strconv.ParseFloat(bidPrice, 64)
	if sellP > bidP {
		// Asking more than best bid → cannot trade → rest as ask.
		return e.Book.AddAsk(ctx, sell.Ticker, sell.ID, sell.Price)
	}

	buy , err := e.Repo.GetOrderByID(ctx, bidID)
	if err != nil {
		return err
	}

	if buy.Status != orders.StatusPending || buy.Quantity != sell.Quantity {
		return e.Book.AddAsk(ctx, sell.Ticker, sell.ID, sell.Price)
	}

	return e.emitMatch(ctx, buy, sell, bidPrice)
}

// if theres a match remove buy and sell order from redis zset and push it to order_stream
func (e *Engine) emitMatch(ctx context.Context, buy orders.Order, sell orders.Order, execPrice string) error {

	fmt.Printf("Order Matched. Pushing order to stream")
	if err := e.Book.Remove(ctx, buy.Ticker, buy.ID); err != nil{
		return err
	}
	if err := e.Book.Remove(ctx, sell.Ticker, sell.ID); err != nil{
		return err
	}

	_ , err := e.Redis.RDB.XAdd(ctx, &redis.XAddArgs{
		Stream: redisx.StreamSettlement,
		Values: map[string]interface{}{
			"trade_id": 		uuid.NewString(),
			"buy_order_id": 	buy.ID,
			"sell_order_id": 	sell.ID,
			"ticker":			buy.Ticker, 
			"price":			execPrice,
			"qty":				buy.Quantity, 
		},
	}).Result()

	return err
}