package engine

import (
	"sync"
)

type Exchange struct {
	books map[string]*OrderBook 
	mu sync.RWMutex
}

func NewExchange() *Exchange {
	return &Exchange{
		books: make(map[string]*OrderBook),
	}
}


func (ex *Exchange) RouteOrder(o *Order) {
	ex.mu.RLock()
	book, exists := ex.books[o.Symbol]
	ex.mu.RUnlock()


	if !exists {
		ex.mu.Lock()


		book, exists = ex.books[o.Symbol]
		if !exists {
			book = NewOrderBook()
			ex.books[o.Symbol] = book
		}

		ex.mu.Unlock()
	}

	book.ProcessOrder(o)
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