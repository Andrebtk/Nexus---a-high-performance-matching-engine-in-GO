package engine

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// orderTemplate holds pre-generated, immutable order parameters. A fresh
// *Order is built from a template on every iteration so that in-place
// mutation by the matching engine (Quantity, linked-list pointers, etc.)
// never corrupts state a later iteration or calibration pass depends on.
type orderTemplate struct {
	Symbol   string
	IsBuy    bool
	Quantity int
	Price    uint64
}

func BenchmarkRouteOrder(b *testing.B) {
	log.SetOutput(io.Discard)

	ex := NewExchange(nil, nil, nil, nil, nil, nil)

	const numTemplates = 1000000
	templates := make([]orderTemplate, numTemplates)
	symbols := []string{"AAPL", "MSFT", "TSLA", "NVDA"}

	for i := 0; i < numTemplates; i++ {
		// FIX: side is now independent of symbol. Previously isBuy came
		// from i%2 while symbol came from i%4 - since 2 divides 4, every
		// symbol only ever saw one side, so zero matches were possible.
		isBuy := rand.Intn(2) == 0
		price := uint64(100 + rand.Intn(20))
		if isBuy {
			price = uint64(90 + rand.Intn(20)) // overlap so matches actually occur
		}
		templates[i] = orderTemplate{
			Symbol:   symbols[rand.Intn(len(symbols))],
			IsBuy:    isBuy,
			Quantity: rand.Intn(100) + 1,
			Price:    price,
		}
	}

	var idCounter int64
	var submittedQty int64

	b.ResetTimer()

	b.Run("Concurrent", func(b *testing.B) {
		b.ReportAllocs()

		var wg sync.WaitGroup
		for i := 0; i < b.N; i++ {
			t := templates[i%numTemplates]
			id := atomic.AddInt64(&idCounter, 1)
			atomic.AddInt64(&submittedQty, int64(t.Quantity))

			// FIX: allocate a fresh Order per iteration instead of reusing
			// pointers into a pre-built array. RouteOrder/ProcessOrder
			// mutate Quantity in place, so a "used" order fed back in on
			// a later iteration (or later calibration pass) isn't
			// equivalent to a new one.
			o := &Order{
				Id:        fmt.Sprintf("bench_%d", id),
				UserID:    "9999",
				Symbol:    t.Symbol,
				IsBuy:     t.IsBuy,
				Quantity:  t.Quantity,
				Price:     t.Price,
				TimeStamp: time.Now(),
			}

			wg.Add(1)
			go func(o *Order) {
				defer wg.Done()
				ex.RouteOrder(o)
			}(o)
		}
		wg.Wait()
	})

	b.StopTimer()

	// Verification: sum remaining resting volume across every book and
	// compare to what was submitted. (submitted - resting) approximates
	// how much quantity actually matched, so you can confirm real
	// matching happened instead of assuming it.
	var restingQty int64
	for _, symbol := range ex.GetTickers() {
		book := ex.GetOrderBook(symbol)
		if book == nil {
			continue
		}
		for _, limit := range book.Bids {
			restingQty += int64(limit.TotalVolume)
		}
		for _, limit := range book.Asks {
			restingQty += int64(limit.TotalVolume)
		}
	}

	b.Logf("submitted qty: %d, resting qty at end: %d, matched (approx): %d",
		submittedQty, restingQty, submittedQty-restingQty)
}
