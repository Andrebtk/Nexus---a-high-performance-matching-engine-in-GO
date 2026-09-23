package engine

import (
	"errors"
	"sync"
	"Nexus/internal/services"
	"Nexus/internal/models"
	"log"
	"strconv"
)

type SettlementJob struct {
	Type              string // "TRADE" or "UPDATE_TAKER_STATUS"
	
	// For TRADE
	MakerOrder        Order
	TakerOrder        Order
	MatchedQuantity   int
	Price             uint64
	
	// For UPDATE_TAKER_STATUS
	TakerRemainingQty int
}

type Exchange struct {
	books map[string]*OrderBook

	userService        *services.UserService
	transactionService *services.TransactionService
	profitLossService  *services.ProfitLossService
	orderService       *services.OrderService
	postgresUserService *services.PostgresUserService
	costBasisService    *services.CostBasisService

	settlementChan chan SettlementJob

	mu sync.RWMutex
}

func NewExchange(userService *services.UserService,
	transactionService *services.TransactionService,
	profitLossService *services.ProfitLossService,
	orderService *services.OrderService,
	postgresUserService *services.PostgresUserService,
	costBasisService *services.CostBasisService) *Exchange {
	ex := &Exchange{
		books: make(map[string]*OrderBook),
		userService:        userService,
		transactionService: transactionService,
		profitLossService:  profitLossService,
		orderService:       orderService,
		postgresUserService: postgresUserService,
		costBasisService:    costBasisService,
		settlementChan:     make(chan SettlementJob, 10000), // Buffer for high throughput
	}
	
	go ex.runSettlementWorker()
	return ex
}

func (e *Exchange) runSettlementWorker() {
	for job := range e.settlementChan {
		e.processSettlementJob(job)
	}
}

func (e *Exchange) getOrCreateBook(symbol string) *OrderBook {
	e.mu.RLock()
	book, ok := e.books[symbol]
	e.mu.RUnlock()
	
	if ok {
		return book
	}
	
	e.mu.Lock()
	defer e.mu.Unlock()
	// Double-check pattern
	book, ok = e.books[symbol]
	if !ok {
		book = NewOrderBook()
		e.books[symbol] = book
	}
	return book
}

func (e *Exchange) RouteOrder(o *Order) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Only log for non-system orders to reduce noise
	if o.DBOrderID > 0 || o.UserID != "system_bot" {
		log.Printf("DEBUG: [EXCHANGE] Starting processing for order %s, symbol=%s, dbOrderID=%d",
			o.Id, o.Symbol, o.DBOrderID)
	}

	// Get or create the order book for the symbol
	book, ok := e.books[o.Symbol]
	if !ok {
		book = NewOrderBook()
		e.books[o.Symbol] = book
	}


	var systemUser *models.User

	if o.UserID == "system_bot" {
		user, err := e.userService.GetUser(o.UserID)
		if err != nil {
			log.Printf("ERROR: system_bot not found in user service: %v", err)
			return
		}
		systemUser = user
		currentBalance := systemUser.Balance
		
		if o.IsBuy {
			requiredBalance := float64(o.Quantity) * float64(o.Price)
			if currentBalance < requiredBalance {
				log.Printf("WARNING: Order %s rejected - insufficient balance for system_bot. Required: $%.2f, Available: $%.2f",
					o.Id, requiredBalance, currentBalance)
				return
			}
		}
	} else {
		_, err := strconv.Atoi(o.UserID)
		if err != nil {
			log.Printf("ERROR: Invalid non-numeric user ID %s", o.UserID)
			return
		}
		// Note: We DO NOT check balance here for real users. 
		// Authorization and balance deduction already happens at the API layer (PlaceOrderHandler).
		// Checking it here would falsely reject orders since the balance is already deducted.
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

	// Only log detailed processing for non-system orders
	if o.DBOrderID > 0 || o.UserID != "system_bot" {
		log.Printf("DEBUG: Processing order %s: symbol=%s, isBuy=%t, quantity=%d, price=%d, user=%s, dbOrderID=%d",
			o.Id, o.Symbol, o.IsBuy, o.Quantity, o.Price, o.UserID, o.DBOrderID)
	}


	fills := book.ProcessOrder(o)

	// Dispatch fills to async settlement worker
	for _, fill := range fills {
		e.settleMakerFill(fill)
	}

	// Only log post-processing details for non-system orders to reduce noise
	if o.DBOrderID > 0 || o.UserID != "system_bot" {
		log.Printf("DEBUG: After processing order %s: remaining quantity=%d, matched quantity=%d",
			o.Id, o.Quantity, originalQuantity - o.Quantity)

		// The order has already been placed in the order book by ProcessOrder if there was remaining quantity
		if o.Quantity > 0 {
			log.Printf("DEBUG: Order %s placed in order book with remaining quantity %d", o.Id, o.Quantity)
		} else {
			log.Printf("DEBUG: Order %s was fully matched, not adding to order book", o.Id)
		}
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
	// Only log order status debug for non-system orders
	if o.DBOrderID > 0 || o.UserID != "system_bot" {
		log.Printf("DEBUG: [ORDER STATUS] Checking order status update for order %s, DBOrderID=%d, Quantity=%d, orderService=%v",
			o.Id, o.DBOrderID, o.Quantity, e.orderService != nil)
	}

	if e.orderService != nil {
		// Debug: Log the order details to help diagnose completion issues
		// Only log completion check debug for non-system orders
		if o.DBOrderID > 0 || o.UserID != "system_bot" {
			log.Printf("DEBUG: [ORDER COMPLETION CHECK] Order %s: Symbol=%s, UserID=%s, DBOrderID=%d, Quantity=%d, IsBuy=%t, OriginalQuantity=%d",
				o.Id, o.Symbol, o.UserID, o.DBOrderID, o.Quantity, o.IsBuy, originalQuantity)
		}
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


func (e *Exchange) processSettlementJob(job SettlementJob) {
	if job.Type == "TRADE" {
		e.settleMakerAsync(job.MakerOrder, job.MatchedQuantity, job.Price)
		e.settleTakerAsync(job.TakerOrder, job.MatchedQuantity, job.Price)
	} else if job.Type == "UPDATE_TAKER_STATUS" {
		e.updateTakerStatusAsync(job.TakerOrder, job.TakerRemainingQty)
	}
}

// settleMakerAsync handles the settlement of maker orders in the async worker
func (e *Exchange) settleMakerAsync(maker Order, matchedQty int, price uint64) {
	tradeAmount := float64(matchedQty) * float64(price)

	log.Printf("INFO: [SETTLE MAKER ASYNC] Processing fill for maker order %s, quantity=%d, price=%d, user=%s, dbOrderID=%d",
		maker.Id, matchedQty, price, maker.UserID, maker.DBOrderID)

	if maker.UserID == "system_bot" {
		user, err := e.userService.GetUser(maker.UserID)
		if err == nil {
			if maker.IsBuy {
				user.Balance -= tradeAmount
			} else {
				user.Balance += tradeAmount
			}
		}
	} else if e.postgresUserService != nil {
		numericUserID, err := strconv.Atoi(maker.UserID)
		if err != nil {
			log.Printf("ERROR: Invalid non-numeric user ID for maker %s", maker.UserID)
			return
		}

		if maker.IsBuy {
			// BUY Maker gets stock and cost basis updated
			err := e.postgresUserService.AddStockOwnership(numericUserID, maker.Symbol, matchedQty)
			if err != nil {
				log.Printf("Warning: Failed to update stock ownership for maker %d: %v", numericUserID, err)
			}
			err = e.costBasisService.RecordBuy(numericUserID, maker.Symbol, matchedQty, float64(price))
			if err != nil {
				log.Printf("Warning: Failed to record cost basis for maker buy order: %v", err)
			}
		} else {
			// SELL Maker receives proceeds, loses stock, gets realized P&L
			err := e.postgresUserService.UpdateUserBalance(numericUserID, tradeAmount)
			if err != nil {
				log.Printf("ERROR: Failed to credit maker %d for sale: %v", numericUserID, err)
			}

			currentQuantity, err := e.postgresUserService.GetStockQuantity(numericUserID, maker.Symbol)
			if err == nil {
				e.postgresUserService.UpdateStockOwnership(numericUserID, maker.Symbol, currentQuantity-matchedQty)
			}

			realized, err := e.costBasisService.RecordSell(numericUserID, maker.Symbol, matchedQty, float64(price))
			if err == nil && realized != 0 {
				e.postgresUserService.AddRealizedPL(numericUserID, realized)
			}
		}
	} else {
		log.Printf("DEBUG: Skipping maker postgres settlement, postgresUserService is nil")
	}

	if e.transactionService != nil {
		transactionType := "trade"
		if maker.IsBuy {
			e.transactionService.RecordTransaction(maker.UserID, maker.Id, transactionType, -tradeAmount)
		} else {
			e.transactionService.RecordTransaction(maker.UserID, maker.Id, transactionType, tradeAmount)
		}
	}

	if e.orderService != nil {
		if maker.Quantity == 0 && maker.DBOrderID > 0 {
			log.Printf("INFO: [MAKER COMPLETION] Maker order %d was fully matched, updating status to completed", maker.DBOrderID)
			err := e.orderService.CompleteOrder(maker.DBOrderID)
			if err != nil {
				log.Printf("Warning: Failed to complete maker order %d: %v", maker.DBOrderID, err)
			}
		} else if maker.Quantity > 0 && maker.DBOrderID > 0 {
			log.Printf("INFO: Maker order %d partially matched, updating remaining quantity to %d", maker.DBOrderID, maker.Quantity)
			err := e.orderService.UpdateOrderQuantity(maker.DBOrderID, int(maker.Quantity))
			if err != nil {
				log.Printf("Warning: Failed to update maker order quantity for order %d: %v", maker.DBOrderID, err)
			}
		}
	}
}

func (e *Exchange) updateTakerStatusAsync(taker Order, remainingQty int) {
	if e.orderService == nil {
		return
	}
	
	if remainingQty == 0 {
		log.Printf("INFO: [ORDER COMPLETION] Order %s was fully matched, initiating completion process", taker.Id)
		if taker.DBOrderID > 0 {
			err := e.orderService.CompleteOrder(taker.DBOrderID)
			if err != nil {
				log.Printf("Warning: Failed to update order status for order %d: %v", taker.DBOrderID, err)
			}
		} else if taker.UserID != "system_bot" {
			// Try to complete by details for numeric user ID if we don't have DBOrderID
			// (Though ideally we should always have DBOrderID)
			numericID, err := strconv.Atoi(taker.UserID)
			if err == nil {
				err = e.orderService.CompleteOrderByDetails(numericID, taker.Symbol, float64(taker.Price), taker.TimeStamp)
				if err != nil {
					log.Printf("Warning: Failed to update order status by details: %v", err)
				}
			}
		}
	} else {
		if taker.DBOrderID > 0 {
			log.Printf("INFO: Order %d partially matched, remaining quantity: %d", taker.DBOrderID, remainingQty)
			err := e.orderService.UpdateOrderQuantity(taker.DBOrderID, remainingQty)
			if err != nil {
				log.Printf("Warning: Failed to update order quantity for order %d: %v", taker.DBOrderID, err)
			}
		}
	}
}

func (e *Exchange) settleTakerAsync(taker Order, matchedQuantity int, price uint64) {
	actualMatchedAmount := float64(matchedQuantity) * float64(price)

	var numericUserID int
	if taker.UserID != "system_bot" {
		id, err := strconv.Atoi(taker.UserID)
		if err == nil {
			numericUserID = id
		}
	}

	// Record transaction
	transactionType := "trade"
	transactionAmount := actualMatchedAmount
	if taker.IsBuy {
		transactionAmount = -transactionAmount
	}
	if e.transactionService != nil {
		e.transactionService.RecordTransaction(taker.UserID, taker.Id, transactionType, transactionAmount)
	}

	if taker.UserID == "system_bot" {
		user, err := e.userService.GetUser(taker.UserID)
		if err == nil {
			if taker.IsBuy {
				// Refund overpayment (locked at limit price, executed at fill price)
				refund := float64(matchedQuantity)*float64(taker.Price) - actualMatchedAmount
				user.Balance += refund
			} else {
				// Receive proceeds
				user.Balance += actualMatchedAmount
			}
		}
	} else if e.postgresUserService != nil {
		if taker.IsBuy {
			// BUY: User already paid matchedQuantity * o.Price in PlaceOrderHandler.
			// Refund them the difference between what they locked and what it actually cost.
			refund := float64(matchedQuantity)*float64(taker.Price) - actualMatchedAmount
			if refund > 0 {
				err := e.postgresUserService.UpdateUserBalance(numericUserID, refund)
				if err != nil {
					log.Printf("ERROR: Failed to refund balance for Postgres user %d: %v", numericUserID, err)
				}
			}

			// Add the matched quantity to user's stock ownership
			log.Printf("INFO: Adding %d shares of %s to user %d's stock ownership", matchedQuantity, taker.Symbol, numericUserID)
			err := e.postgresUserService.AddStockOwnership(numericUserID, taker.Symbol, matchedQuantity)
			if err != nil {
				log.Printf("Warning: Failed to update stock ownership for user %d: %v", numericUserID, err)
			}

			// Record cost basis
			avgPrice := actualMatchedAmount / float64(matchedQuantity)
			err = e.costBasisService.RecordBuy(numericUserID, taker.Symbol, matchedQuantity, avgPrice)
			if err != nil {
				log.Printf("Warning: Failed to record cost basis for buy order: %v", err)
			}
		} else {
			// SELL: Credit the user's balance with the proceeds from the sale
			log.Printf("INFO: Crediting user %d with $%.2f from sale of %d shares of %s", numericUserID, actualMatchedAmount, matchedQuantity, taker.Symbol)
			err := e.postgresUserService.UpdateUserBalance(numericUserID, actualMatchedAmount)
			if err != nil {
				log.Printf("ERROR: Failed to credit user %d for sale: %v", numericUserID, err)
			}

			// Subtract the matched quantity from user's stock ownership
			log.Printf("INFO: Removing %d shares of %s from user %d's stock ownership", matchedQuantity, taker.Symbol, numericUserID)
			currentQuantity, err := e.postgresUserService.GetStockQuantity(numericUserID, taker.Symbol)
			if err != nil {
				log.Printf("Warning: Failed to get current stock ownership for user %d: %v", numericUserID, err)
				currentQuantity = 0
			}
			newQuantity := currentQuantity - matchedQuantity
			err = e.postgresUserService.UpdateStockOwnership(numericUserID, taker.Symbol, newQuantity)
			if err != nil {
				log.Printf("Warning: Failed to update stock ownership for user %d: %v", numericUserID, err)
			}

			// Calculate realized P&L and update profit/loss
			avgPrice := actualMatchedAmount / float64(matchedQuantity)
			realized, err := e.costBasisService.RecordSell(numericUserID, taker.Symbol, matchedQuantity, avgPrice)
			if err != nil {
				log.Printf("Warning: Failed to record cost basis for sell order: %v", err)
			} else if realized != 0 {
				err := e.postgresUserService.AddRealizedPL(numericUserID, realized)
				if err != nil {
					log.Printf("Warning: Failed to update realized P&L: %v", err)
				}
			}
		}
	}
}

// RestoreOrder directly places a previously active order back into the order book
// without performing balance checks or immediate matching. This is used during
// system startup to hydrate the in-memory engine from the database.
func (e *Exchange) RestoreOrder(o *Order) {
	e.mu.Lock()
	defer e.mu.Unlock()

	book, ok := e.books[o.Symbol]
	if !ok {
		book = NewOrderBook()
		e.books[o.Symbol] = book
	}

	book.placeMakerOrder(o)
	log.Printf("INFO: [RESTORE] Restored order %s (DB ID: %d) to order book", o.Id, o.DBOrderID)
}

// settleMakerFill handles the settlement of maker orders that have been matched
func (e *Exchange) settleMakerFill(fill Fill) {
	maker := fill.MakerOrder
	matchedQty := fill.Quantity
	tradeAmount := float64(matchedQty) * float64(fill.Price)

	log.Printf("INFO: [SETTLE MAKER] Processing fill for maker order %s, quantity=%d, price=%d, user=%s, dbOrderID=%d",
		maker.Id, matchedQty, fill.Price, maker.UserID, maker.DBOrderID)

	if maker.UserID == "system_bot" {
		user, err := e.userService.GetUser(maker.UserID)
		if err == nil {
			if maker.IsBuy {
				user.Balance -= tradeAmount
			} else {
				user.Balance += tradeAmount
			}
		}
		
		transactionType := "trade"
		if maker.IsBuy {
			e.transactionService.RecordTransaction(maker.UserID, maker.Id, transactionType, -tradeAmount)
		} else {
			e.transactionService.RecordTransaction(maker.UserID, maker.Id, transactionType, tradeAmount)
		}
		
		if maker.Quantity == 0 && maker.DBOrderID > 0 {
			e.orderService.CompleteOrder(maker.DBOrderID)
		}
		return
	}

	numericUserID, err := strconv.Atoi(maker.UserID)
	if err != nil || numericUserID <= 0 {
		log.Printf("ERROR: Invalid non-numeric user ID for maker %s: %s", maker.Id, maker.UserID)
		return
	}

	if true { // Keep the indentation block
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
		} else if maker.Quantity > 0 && maker.DBOrderID > 0 {
			log.Printf("INFO: Maker order %d partially matched, updating remaining quantity to %d", maker.DBOrderID, maker.Quantity)
			err := e.orderService.UpdateOrderQuantity(maker.DBOrderID, int(maker.Quantity))
			if err != nil {
				log.Printf("Warning: Failed to update maker order quantity for order %d: %v", maker.DBOrderID, err)
			}
		} else if maker.Quantity == 0 {
			// Handle orders with DBOrderID=0 (system_bot or failed creation)
			log.Printf("WARNING: [MAKER COMPLETION] Maker order %s was fully matched but has DBOrderID=0", maker.Id)
			// we don't try to complete system bot orders by details here because system_bot is handled above
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
