package engine

import (
	"errors"
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
	postgresUserService *services.PostgresUserService
	costBasisService    *services.CostBasisService

	mu sync.RWMutex
}

func NewExchange(userService *services.UserService,
	transactionService *services.TransactionService,
	profitLossService *services.ProfitLossService,
	orderService *services.OrderService,
	postgresUserService *services.PostgresUserService,
	costBasisService *services.CostBasisService) *Exchange {
	return &Exchange{
		books: make(map[string]*OrderBook),
		userService:        userService,
		transactionService: transactionService,
		profitLossService:  profitLossService,
		orderService:       orderService,
		postgresUserService: postgresUserService,
		costBasisService:    costBasisService,
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
            log.Printf("WARNING: Order %s rejected - insufficient balance. Required: $%.2f, Available: $%.2f",
                o.Id, requiredBalance, user.Balance)
            return
        }
    }

	// Store the original quantity before processing
	originalQuantity := o.Quantity

	log.Printf("DEBUG: Processing order %s: symbol=%s, isBuy=%t, quantity=%d, price=%d, user=%s, dbOrderID=%d",
		o.Id, o.Symbol, o.IsBuy, o.Quantity, o.Price, o.UserID, o.DBOrderID)

	fills := book.ProcessOrder(o)

	// Process fills to settle maker orders
	for _, fill := range fills {
		e.settleMakerFill(fill)
	}

	log.Printf("DEBUG: After processing order %s: remaining quantity=%d, matched quantity=%d",
		o.Id, o.Quantity, originalQuantity - o.Quantity)

	// The order has already been placed in the order book by ProcessOrder if there was remaining quantity
	if o.Quantity > 0 {
		log.Printf("DEBUG: Order %s placed in order book with remaining quantity %d", o.Id, o.Quantity)
	} else {
		log.Printf("DEBUG: Order %s was fully matched, not adding to order book", o.Id)
	}

    // Record transaction
    transactionType := "trade"
    amount := float64(originalQuantity - o.Quantity) * float64(o.Price)
    if !o.IsBuy {
        amount = -amount
    }
    // For PostgreSQL users, ensure we use the same user ID format as the user service
    transactionUserID := o.UserID
    if numericUserID, err := strconv.Atoi(o.UserID); err == nil && numericUserID > 0 {
        // For PostgreSQL users, use the numeric ID to match user service format
        transactionUserID = strconv.Itoa(numericUserID)
    }
    e.transactionService.RecordTransaction(transactionUserID, o.Id, transactionType, amount)

	// Update user balance
	if o.IsBuy {
		user.Balance -= float64(originalQuantity - o.Quantity) * float64(o.Price)
	} else {
		user.Balance += float64(originalQuantity - o.Quantity) * float64(o.Price)
	}

	// Track cost basis and realized P&L for PostgreSQL users
	matchedQuantity := originalQuantity - o.Quantity
	if matchedQuantity > 0 {
		if numericUserID, err := strconv.Atoi(o.UserID); err == nil && numericUserID > 0 {
			if o.IsBuy {
				// For buy orders, record the cost basis
				err := e.costBasisService.RecordBuy(numericUserID, o.Symbol, matchedQuantity, float64(o.Price))
				if err != nil {
					log.Printf("Warning: Failed to record cost basis for buy order: %v", err)
				}
			} else {
				// For sell orders, calculate realized P&L and update profit/loss
				realized, err := e.costBasisService.RecordSell(numericUserID, o.Symbol, matchedQuantity, float64(o.Price))
				if err != nil {
					log.Printf("Warning: Failed to record cost basis for sell order: %v", err)
				} else if realized != 0 {
					// Update realized P&L
					err := e.postgresUserService.AddRealizedPL(numericUserID, realized)
					if err != nil {
						log.Printf("Warning: Failed to update realized P&L: %v", err)
					} else {
						log.Printf("INFO: Realized P&L for user %d: $%.2f (symbol: %s, quantity: %d, sellPrice: $%.2f)",
							numericUserID, realized, o.Symbol, matchedQuantity, float64(o.Price))
					}
				}
			}
		}
	}

	// Update order status in database if this is a PostgreSQL user
	log.Printf("DEBUG: [ORDER STATUS] Checking order status update for order %s, DBOrderID=%d, Quantity=%d, orderService=%v",
		o.Id, o.DBOrderID, o.Quantity, e.orderService != nil)

	if e.orderService != nil {
		// Debug: Log the order details to help diagnose completion issues
		log.Printf("DEBUG: [ORDER COMPLETION CHECK] Order %s: Symbol=%s, UserID=%s, DBOrderID=%d, Quantity=%d, IsBuy=%t, OriginalQuantity=%d",
			o.Id, o.Symbol, o.UserID, o.DBOrderID, o.Quantity, o.IsBuy, originalQuantity)
		// Check if this order was fully matched (quantity is 0)
		if o.Quantity == 0 {
			log.Printf("INFO: [ORDER COMPLETION] Order %s was fully matched (quantity=%d), initiating completion process", o.Id, o.Quantity)
			// Mark the order as completed in the database
			if o.DBOrderID > 0 {
				log.Printf("INFO: Order %d was fully matched, updating status to completed", o.DBOrderID)
				err := e.orderService.CompleteOrder(o.DBOrderID)
				if err != nil {
					log.Printf("Warning: Failed to update order status for order %d: %v", o.DBOrderID, err)
				} else {
					log.Printf("INFO: Successfully updated order %d status to completed", o.DBOrderID)
				}
			} else {
				// Handle orders with DBOrderID=0 (system_bot or failed creation)
				log.Printf("WARNING: Order %s was fully matched but has DBOrderID=0, attempting alternative status update", o.Id)
				// Try to find and update the order by other identifiers
				if numericUserID, err := strconv.Atoi(o.UserID); err == nil && numericUserID > 0 {
					// Try to update by user ID, symbol, price, and timestamp
					err := e.orderService.CompleteOrderByDetails(numericUserID, o.Symbol, float64(o.Price), o.TimeStamp)
					if err != nil {
						log.Printf("Warning: Failed to update order status for system_bot order: %v", err)
					} else {
						log.Printf("INFO: Successfully updated system_bot order status using alternative method")
					}
				}
			}

			// Update stock ownership for completed orders
			// Convert userID to integer for PostgreSQL users
			if numericUserID, err := strconv.Atoi(o.UserID); err == nil && numericUserID > 0 && e.postgresUserService != nil {
				matchedQuantity := originalQuantity - o.Quantity
				matchedAmount := float64(matchedQuantity) * float64(o.Price)

				if o.IsBuy {
					// For buy orders, add the matched quantity to user's stock ownership
					log.Printf("INFO: Adding %d shares of %s to user %d's stock ownership", matchedQuantity, o.Symbol, numericUserID)
					err := e.postgresUserService.AddStockOwnership(numericUserID, o.Symbol, matchedQuantity)
					if err != nil {
						log.Printf("Warning: Failed to update stock ownership for user %d: %v", numericUserID, err)
					}
				} else {
					// For sell orders, subtract the matched quantity from user's stock ownership
					// AND credit the user's balance with the proceeds
					log.Printf("INFO: Removing %d shares of %s from user %d's stock ownership", matchedQuantity, o.Symbol, numericUserID)
					// Get current ownership and subtract
					currentQuantity, err := e.postgresUserService.GetStockQuantity(numericUserID, o.Symbol)
					if err != nil {
						log.Printf("Warning: Failed to get current stock ownership for user %d: %v", numericUserID, err)
						currentQuantity = 0
					}
					newQuantity := currentQuantity - matchedQuantity
					err = e.postgresUserService.UpdateStockOwnership(numericUserID, o.Symbol, newQuantity)
					if err != nil {
						log.Printf("Warning: Failed to update stock ownership for user %d: %v", numericUserID, err)
					}

					// Credit the user's balance with the proceeds from the sale
					log.Printf("INFO: Crediting user %d with $%.2f from sale of %d shares of %s", numericUserID, matchedAmount, matchedQuantity, o.Symbol)
					err = e.postgresUserService.UpdateUserBalance(numericUserID, matchedAmount)
					if err != nil {
						log.Printf("Warning: Failed to credit user %d for sale: %v", numericUserID, err)
					}
				}
			}
		} else {
			// Ensure the order status is set to 'active' if it's not fully matched
			if o.DBOrderID > 0 {
				log.Printf("INFO: Order %d partially matched, remaining quantity: %d", o.DBOrderID, o.Quantity)
			} else {
				log.Printf("INFO: Order %s partially matched, remaining quantity: %d", o.Id, o.Quantity)
			}
			// No need to update status here as it should already be 'active'
		}
	} else {
		log.Printf("WARNING: Order %s cannot update status - orderService is nil", o.Id)
	}
}






// settleMakerFill handles the settlement of maker orders that have been matched
func (e *Exchange) settleMakerFill(fill Fill) {
	maker := fill.MakerOrder
	matchedQty := fill.Quantity
	tradeAmount := float64(matchedQty) * float64(fill.Price)

	log.Printf("INFO: [SETTLE MAKER] Processing fill for maker order %s, quantity=%d, price=%d, user=%s, dbOrderID=%d",
		maker.Id, matchedQty, fill.Price, maker.UserID, maker.DBOrderID)

	// Check if this is a PostgreSQL user (numeric user ID)
	if numericUserID, err := strconv.Atoi(maker.UserID); err == nil && numericUserID > 0 {
		if maker.IsBuy {
			// Maker was buying: record cost basis and update stock ownership
			err := e.costBasisService.RecordBuy(numericUserID, maker.Symbol, matchedQty, float64(fill.Price))
			if err != nil {
				log.Printf("Warning: Failed to record cost basis for maker buy order: %v", err)
			}

			// Update stock ownership
			err = e.postgresUserService.AddStockOwnership(numericUserID, maker.Symbol, matchedQty)
			if err != nil {
				log.Printf("Warning: Failed to update stock ownership for maker buy: %v", err)
			}
		} else {
			// Maker was selling: calculate realized P&L and update balance
			realized, err := e.costBasisService.RecordSell(numericUserID, maker.Symbol, matchedQty, float64(fill.Price))
			if err != nil {
				log.Printf("Warning: Failed to record cost basis for maker sell order: %v", err)
			} else if realized != 0 {
				// Update realized P&L
				err := e.postgresUserService.AddRealizedPL(numericUserID, realized)
				if err != nil {
					log.Printf("Warning: Failed to update realized P&L for maker: %v", err)
				} else {
					log.Printf("INFO: Realized P&L for maker %d: $%.2f (symbol: %s, quantity: %d, sellPrice: $%.2f)",
						numericUserID, realized, maker.Symbol, matchedQty, float64(fill.Price))
				}
			}

			// Update stock ownership
			currentQuantity, err := e.postgresUserService.GetStockQuantity(numericUserID, maker.Symbol)
			if err != nil {
				log.Printf("Warning: Failed to get current stock ownership for maker: %v", err)
				currentQuantity = 0
			}
			newQuantity := currentQuantity - matchedQty
			err = e.postgresUserService.UpdateStockOwnership(numericUserID, maker.Symbol, newQuantity)
			if err != nil {
				log.Printf("Warning: Failed to update stock ownership for maker sell: %v", err)
			}

			// Credit the maker's balance with the proceeds from the sale
			err = e.postgresUserService.UpdateUserBalance(numericUserID, tradeAmount)
			if err != nil {
				log.Printf("Warning: Failed to credit maker for sale: %v", err)
			}
		}

		// Record transaction for the maker
		transactionType := "trade"
		if maker.IsBuy {
			// Maker was buying, so this is a negative transaction (money spent)
			e.transactionService.RecordTransaction(maker.UserID, maker.Id, transactionType, -tradeAmount)
		} else {
			// Maker was selling, so this is a positive transaction (money received)
			e.transactionService.RecordTransaction(maker.UserID, maker.Id, transactionType, tradeAmount)
		}

		// This is the KEY FIX: Mark the maker order as completed if it's fully matched
		if maker.Quantity == 0 && maker.DBOrderID > 0 {
			log.Printf("INFO: [MAKER COMPLETION] Maker order %d was fully matched, updating status to completed", maker.DBOrderID)
			err := e.orderService.CompleteOrder(maker.DBOrderID)
			if err != nil {
				log.Printf("Warning: Failed to complete maker order %d: %v", maker.DBOrderID, err)
			} else {
				log.Printf("INFO: Successfully completed maker order %d", maker.DBOrderID)
			}
		} else if maker.Quantity == 0 {
			// Handle orders with DBOrderID=0 (system_bot or failed creation)
			log.Printf("WARNING: [MAKER COMPLETION] Maker order %s was fully matched but has DBOrderID=0", maker.Id)
			if numericUserID, err := strconv.Atoi(maker.UserID); err == nil && numericUserID > 0 {
				err := e.orderService.CompleteOrderByDetails(numericUserID, maker.Symbol, float64(fill.Price), maker.TimeStamp)
				if err != nil {
					log.Printf("Warning: Failed to complete system_bot maker order: %v", err)
				} else {
					log.Printf("INFO: Successfully completed system_bot maker order using alternative method")
				}
			}
		}
	} else {
		log.Printf("DEBUG: [SETTLE MAKER] Maker order %s belongs to non-PostgreSQL user (userID=%s), skipping settlement", maker.Id, maker.UserID)
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

// CancelOrder removes a resting order for the given symbol from the
// matching engine. It does not touch the database — callers are
// responsible for persisting the cancellation and any refunds.
func (ex *Exchange) CancelOrder(symbol string, dbOrderID int) (*Order, error) {
	ex.mu.RLock()
	book, ok := ex.books[symbol]
	ex.mu.RUnlock()

	if !ok {
		return nil, errors.New("order book not found for symbol")
	}

	order := book.CancelOrder(dbOrderID)
	if order == nil {
		return nil, errors.New("order not found in order book (it may already be filled)")
	}

	return order, nil
}
