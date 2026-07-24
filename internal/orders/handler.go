package orders

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/zuther77/distributed-ledger/internal/metrics"
	"github.com/zuther77/distributed-ledger/internal/redisx"
)


type Handlers struct {
	Repo *Repository
	Redis *redisx.Client
}


// create order. Parse incoming json, insert in postgres, add to redis stream
// handles POST /api/v1/orders
func (h *Handlers) CreateOrder(requestContext *gin.Context) {
	var req CreateOrderRequest
	if err := requestContext.ShouldBindJSON(&req); err != nil {
		requestContext.JSON(http.StatusBadRequest , gin.H{"Error":"Invalid JSON body"} )
		return 
	}

	// validate request before adding to db and stream
	if err := validateOrderRequest(req); err != nil {
		requestContext.JSON(http.StatusBadRequest, gin.H{"Error": err.Error()})
		return
	}

	order := Order{
		ID:  		uuid.NewString(), 
		UserID: 	req.UserID,
		Ticker: 	strings.TrimSpace(strings.ToUpper(req.Ticker)),
		Side: 		Side(strings.ToUpper(req.Side)),
		Quantity: 	req.Quantity,
		Price: 		req.Price,
		Status: 	StatusPending,	
	}

	// add order to POSTGRES first 
	if err := h.Repo.InsertPending(requestContext.Request.Context(), order); err != nil {
		requestContext.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save order in postgres: "})
		return
	}

	// add order to redis stream
	if err := h.Redis.EnqueueOrder(requestContext.Request.Context(), order.ID); err != nil {
		requestContext.JSON(http.StatusInternalServerError, gin.H{"error":"order failed to pused to redis stream. Order saved to postgres"})
		return
	}

	metrics.OrderProcessed.Inc()
	requestContext.JSON(http.StatusCreated , gin.H{
		"id": 		order.ID,
		"status": 	order.Status,
	})

}


// get order by ID
// handle GET /api/v1/orders/:id
func (h *Handlers) GetOrder(c *gin.Context) {
	id := c.Param("id")
	order , err := h.Repo.GetOrderByID(c.Request.Context(), id)
	if errors.Is(err , ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error" : "order not found"})
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load order from Postgres at api handler level"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id" : order.ID, 
		"user_id" : order.UserID, 
		"ticker":    order.Ticker,
		"side":      order.Side,
		"qty":       order.Quantity,
		"price":     order.Price,
		"status":    order.Status,
		"created_at": order.CreatedAt,
	})
}



// valide a order 
func validateOrderRequest(req CreateOrderRequest) error {

	// check if user_id is present
	if strings.TrimSpace(req.UserID) == "" {
		return errors.New("user_id is required")
	}

	// check if ticker is present
	if strings.TrimSpace(req.Ticker) == "" {
		return errors.New("ticker is required")
	}

	// check if side is BUY or SELL only 
	side := strings.ToUpper(strings.TrimSpace(req.Side))
	if side != string(SideBuy) && side != string(SideSell) {
		return errors.New("side must be BUY or SELL")
	}

	// check quantity 
	if strings.TrimSpace(req.Quantity) == "" {
		return errors.New("quantity is required")
	}

	// price check 
	if strings.TrimSpace(req.Price) == "" {
		return errors.New("price is required")
	}

	if req.Quantity[0] == '-' || req.Price[0] == '-' {
		return errors.New("quantity and price must be positive")
	}

	return nil

}