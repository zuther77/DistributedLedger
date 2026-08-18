package orderbook

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zuther77/distributed-ledger/internal/matching"
	"github.com/zuther77/distributed-ledger/internal/redisx"
)

type Handler struct {
	Redis *redisx.Client
}

// Get handles GET /api/v1/orderbook/:ticker
func (h *Handler) Get(c *gin.Context) {
	ticker := strings.TrimSpace(strings.ToUpper(c.Param("ticker")))
	if ticker == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticker is required"})
		return
	}

	book := &matching.Book{RDB: h.Redis}
	ctx := c.Request.Context()

	bids, err := book.ListBids(ctx, ticker, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read bids"})
		return
	}
	asks, err := book.ListAsks(ctx, ticker, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read asks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"ticker": ticker,
		"bids":   bids,
		"asks":   asks,
	})
}
