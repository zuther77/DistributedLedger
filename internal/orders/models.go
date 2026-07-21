package orders

import "time"

// Model for Side. BUY or SELL
type Side string
const (
	SideBuy Side  = "BUY"
	SideSell Side = "SELL"
)

// Model for Status. Tracks lifecycle in Postgres
type Status string
const (
	StatusPending  Status = "PENDING"
	StatusFilled   Status = "FILLED"
	StatusCanceled Status = "CANCELED"
)

// Model for Order. similar to row of orders table in pg
type Order struct {
	ID		 	string
	UserID   	string
	Ticker  	string
	Side     	Side
	Quantity 	string
	Price		string
	Status		Status
	CreatedAt	time.Time
}


// CreateOrderRequest is what the HTTP client sends (JSON).
// `json:"..."` tags tell Gin/encoding/json which field names to expect.
type CreateOrderRequest struct {
	UserID		string `json:"user_id"` 
	Ticker		string `json:"ticker"`
	Side 		string `json:"side"`
	Quantity	string `json:"qty"`
	Price		string `json:"price"`
}