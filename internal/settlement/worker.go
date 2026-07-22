package settlement

import (
	"context"
	"fmt"

	"github.com/zuther77/distributed-ledger/internal/db"
	"github.com/zuther77/distributed-ledger/internal/matching"
)

type Worker struct {
	DB *db.Pool
}

// Settlement is idempotent: safe if Redis redelivers the same settlement message.
func (worker * Worker) Settle(ctx context.Context, event matching.MatchEvent) error {
	transaction , err := worker.DB.Conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer transaction.Rollback(ctx)
	const updateOrderquery = `UPDATE orders SET status='FILLED' WHERE id=$1 AND status='PENDING'`
	buyUpdateRes , err := transaction.Exec(ctx, updateOrderquery, event.BuyOrderID)
	if err != nil {
		return err
	}
	sellUpdateRes, err := transaction.Exec(ctx, updateOrderquery, event.SellOrderID)
	if err != nil {
		return err
	}

	if buyUpdateRes.RowsAffected() == 0 || sellUpdateRes.RowsAffected() == 0 {
		// already processed ( or order is gone ). Still "success" for ACK purpose
		return transaction.Commit(ctx)
	}
	
	// lock users rows so two settlements cannot interleave balance math
	var buyerUserID, sellerUserID string
	const lockUserQuery = `SELECT user_id FROM orders WHERE id=$1 FOR UPDATE`
	if err := transaction.QueryRow(ctx, lockUserQuery, event.BuyOrderID,
	).Scan(&buyerUserID); err != nil {
		return err
	}
	if err := transaction.QueryRow(ctx, lockUserQuery,  event.SellOrderID,
	).Scan(&sellerUserID); err != nil {
		return err
	}

	// update user balance 
	const (
		updateBalanceBuy  = `UPDATE users SET balance = balance - ($1::numeric * $2::numeric) WHERE id=$3`
		updateBalanceSell = `UPDATE users SET balance = balance + ($1::numeric * $2::numeric) WHERE id=$3`
	)
	if _, err := transaction.Exec(ctx, updateBalanceBuy, event.Price, event.Qty, buyerUserID); err != nil {
			return err
	}
	if _, err := transaction.Exec(ctx, updateBalanceSell, event.Price, event.Qty, sellerUserID); err != nil {
		return err
	}
	

	// ON CONFLICT DO Nothing: if trade_id somehow retires after insert, ok
	const insertTradeQuery = `INSERT INTO trades (id, buy_order_id, sell_order_id, ticker, execution_price, quantity)
							VALUES ($1, $2, $3, $4, $5::numeric, $6::numeric)
							ON CONFLICT (id) DO NOTHING`
	if _, err := transaction.Exec(ctx, insertTradeQuery, event.TradeID, event.BuyOrderID, event.SellOrderID, event.Ticker, event.Price, event.Qty); err != nil {
		return err
	}
	

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("Commit to PG faild: %w", err)
	}

	return nil
}


// map Redis values to MatchEvent
func ParseEvent(messageFields map[string]interface{}) (matching.MatchEvent, error) {
	readField := func(fieldName string) string {
		fieldValue , _ := messageFields[fieldName].(string)
		return fieldValue
	}

	matchEvent := matching.MatchEvent{
		TradeID: 	readField("trade_id"),
		BuyOrderID: readField("buy_order_id"),
		SellOrderID: readField("sell_order_id"),
		Ticker: readField("ticker"),
		Price: readField("price"),
		Qty: readField("qty"),
	}

	if matchEvent.TradeID == "" || matchEvent.BuyOrderID == "" || matchEvent.SellOrderID == "" {
		return matchEvent, fmt.Errorf("Incomplete Settlement event: %+v", messageFields)
	}
	
	return matchEvent, nil
}