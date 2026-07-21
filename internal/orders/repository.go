package orders

import (
	"context"
	"errors"
	"fmt"
	"time"
	"github.com/jackc/pgx/v5"
	"github.com/zuther77/distributed-ledger/internal/db"
)

// 404 order not found
var ErrNotFound = errors.New("order not found")

// Repository talks to postgres
type Repository struct {
	DB *db.Pool
}


// writes a new order with status PENDING
func ( r *Repository) InsertPending(ctx context.Context, o Order) error {

	// sql query
	const query = `INSERT INTO orders (id, user_id, ticker, side, quantity, price, status) 
	VALUES ($1, $2, $3, $4, $5::numeric, $6::numeric, $7)`

	_ , err := r.DB.Conn.Exec(ctx, query, 
		o.ID, 
		o.UserID,
		o.Ticker,
		string(o.Side),
		o.Quantity,
		o.Price,
		string(o.Status),
	)

	if err != nil {
		return fmt.Errorf("irror order: %w", err)
	}

	return nil
}


// load one order. Return ErrNotFound if missing
func (r * Repository) GetOrderByID(ctx context.Context, id string) (Order , error) {
	
	const query = `SELECT * FROM orders where id = $1`

	var o Order
	var side ,status string

	err := r.DB.Conn.QueryRow(ctx, query,id).Scan(
		&o.ID,
		&o.UserID,
		&o.Ticker,
		&side, 
		&o.Quantity,
		&o.Price,
		&status,
		&o.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return Order{} , ErrNotFound
	}

	o.Side = Side(side)
	o.Status = Status(status)

	return o, nil
}

// return PENDING orders older than age 
func (r * Repository) ListStalePending(ctx context.Context, age time.Duration) ([]Order, error) {
	
	const query=` SELECT id, user_id, ticker, side, quantity::text, price::text, status, created_at
					FROM orders
					WHERE status = 'PENDING' AND created_at < now() - make_interval(secs => $1)
					ORDER BY created_at ASC
					LIMIT 100
				`
	rows , err := r.DB.Conn.Query(ctx, query, int(age.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("List stale pending: %w", err)
	}
	defer rows.Close()

	var output []Order
	for rows.Next() {
		var order Order
		var side , status string
		if err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.Ticker,
			&side, 
			&order.Quantity, 
			&order.Price,
			&status,
			&order.CreatedAt,
		); err != nil {
			return nil, err
		}

		order.Side = Side(side)
		order.Status = Status(status)
		output = append(output, order)
	}

	return output, rows.Err()
}