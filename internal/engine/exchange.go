package engine

import (
	"sync"
	"Nexus/internal/services"
	"Nexus/internal/models"
	"log"
	"strconv"
)

type Exchange struct {
	books map[string]*OrderBook

	userService        *services.UserService
	transactionService *services.TransactionService
	profitLossService  *services.ProfitLossService
	orderService       *services.OrderService

	mu sync.RWMutex
}

func NewExchange(userService *services.UserService,
	transactionService *services.TransactionService,
	profitLossService *services.ProfitLossService,
	orderService *services.OrderService) *Exchange {
	return &Exchange{
		books: make(map[string]*OrderBook),
		userService:        userService,
		transactionService: transactionService,
		profitLossService:  profitLossService,
		orderService:       orderService,
	}
}

func (e *Exchange) RouteOrder(o *Order) {
	e.mu.Lock()
	defer e.mu.Unlock()

	log.Printf("DEBUG: [EXCHANGE] Starting processing for order %s, symbol=%s, dbOrderID=%d",
		o.Id, o.Symbol, o.DBOrderID)

	// Get or create the order book for the symbol
	book, ok := e.books[o.Symbol]
	if !ok {
		book = NewOrderBook()
		e.books[o.Symbol] = book
	}


	user, err := e.userService.GetUser(o.UserID)
	if err != nil {
		// If this is a numeric user ID (PostgreSQL user), create a temporary user
		if _, err2 := strconv.Atoi(o.UserID); err2 == nil {
			log.Printf("WARNING: User %s not found in in-memory service, creating temporary user for PostgreSQL user", o.UserID)
			// Create a temporary user object to allow processing
			user = &models.User{
				ID:      o.UserID,
				Balance: 1000000, // Default high balance to allow processing
			}
		} else {
			log.Printf("ERROR: User %s not found in user service: %v", o.UserID, err)
			return
		}
	}



	if o.IsBuy {
		requiredBalance := float64(o.Quantity) * float64(o.Price)
		if user.Balance < requiredBalance {
			// Handle insufficient balance
			log.Printf("WARNING: Order %s rejected - insufficient balance. Required: $%.2f, Available: $%.2f",
				o.Id, requiredBalance, user.Balance)
			return
		}
	}

	// Store the original quantity before processing
	originalQuantity := o.Quantity

	log.Printf("DEBUG: Processing order %s: symbol=%s, isBuy=%t, quantity=%d, price=%d, user=%s, dbOrderID=%d",
		o.Id, o.Symbol, o.IsBuy, o.Quantity, o.Price, o.UserID, o.DBOrderID)

	book.ProcessOrder(o)

	log.Printf("DEBUG: After processing order %s: remaining quantity=%d, matched quantity=%d",
		o.Id, o.Quantity, originalQuantity - o.Quantity)

	// Place remaining quantity as a maker order if any
	if o.Quantity > 0 {
		log.Printf("DEBUG: Placing order %s in order book with remaining quantity %d", o.Id, o.Quantity)
		book.placeMakerOrder(o)
		log.Printf("DEBUG: Order %s placed in order book successfully", o.Id)
	} else {
		log.Printf("DEBUG: Order %s was fully matched, not adding to order book", o.Id)
	}

	// Record transaction
	transactionType := "trade"
	amount := float64(originalQuantity - o.Quantity) * float64(o.Price)
	if !o.IsBuy {
		amount = -amount
	}
	e.transactionService.RecordTransaction(o.UserID, o.Id, transactionType, amount)

	// Update user balance
	if o.IsBuy {
		user.Balance -= float64(originalQuantity - o.Quantity) * float64(o.Price)
	} else {
		user.Balance += float64(originalQuantity - o.Quantity) * float64(o.Price)
	}

	// Calculate profit/loss
	e.profitLossService.CalculateProfitLoss(o.UserID)

	// Update order status in database if this is a PostgreSQL user
	if e.orderService != nil && o.DBOrderID > 0 {
		// Check if this order was fully matched (quantity is 0)
		if o.Quantity == 0 {
			// Mark the order as completed in the database
			log.Printf("INFO: Order %d was fully matched, updating status to completed", o.DBOrderID)
			err := e.orderService.CompleteOrder(o.DBOrderID)
			if err != nil {
				log.Printf("Warning: Failed to update order status for order %d: %v", o.DBOrderID, err)
			} else {
				log.Printf("INFO: Successfully updated order %d status to completed", o.DBOrderID)
			}
		} else {
			log.Printf("INFO: Order %d partially matched, remaining quantity: %d", o.DBOrderID, o.Quantity)
		}
	}
}






func (ex *Exchange) GetTickers() []string {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	tickers := make([]string, 0, len(ex.books))

	for symbol := range ex.books {
		tickers = append(tickers, symbol)
	}

	return tickers

}

func (ex *Exchange) GetOrderBook(symbol string) *OrderBook {
	ex.mu.RLock()
	defer ex.mu.RUnlock()

	ob, exists := ex.books[symbol]
	if !exists {
		return nil
	}
	return ob
}