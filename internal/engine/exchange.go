package engine

import (
	"sync"
	"Nexus/internal/services"
)

type Exchange struct {
	books map[string]*OrderBook

	userService        *services.UserService
	transactionService *services.TransactionService
	profitLossService  *services.ProfitLossService

	mu sync.RWMutex
}

func NewExchange(userService *services.UserService, 
	transactionService *services.TransactionService, 
	profitLossService *services.ProfitLossService) *Exchange {
	return &Exchange{
		books: make(map[string]*OrderBook),
		userService:        userService,
		transactionService: transactionService,
		profitLossService:  profitLossService,
	}
}

func (e *Exchange) RouteOrder(o *Order) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Get or create the order book for the symbol
	book, ok := e.books[o.Symbol]
	if !ok {
		book = NewOrderBook()
		e.books[o.Symbol] = book
	}


	user, err := e.userService.GetUser(o.UserID)
	if err != nil {
		// Handle error
		return
	}



	if o.IsBuy {
		if user.Balance < float64(o.Quantity) * float64(o.Price) {
			// Handle insufficient balance
			return
		}
		book.matchBuy(o)
	} else {
		book.matchSell(o)
	}

	// Place remaining quantity as a maker order if any
	if o.Quantity > 0 {
		book.placeMakerOrder(o)
	}

	book.ProcessOrder(o)

	// Record transaction
	transactionType := "trade"
	amount := float64(o.Quantity) * float64(o.Price)
	if !o.IsBuy {
		amount = -amount
	}
	e.transactionService.RecordTransaction(o.UserID, o.Id, transactionType, amount)

	// Update user balance
	if o.IsBuy {
		user.Balance -= float64(o.Quantity) * float64(o.Price)
	} else {
		user.Balance += float64(o.Quantity) * float64(o.Price)
	}

	// Calculate profit/loss
	e.profitLossService.CalculateProfitLoss(o.UserID)
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