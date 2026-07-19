package engine 

import (
	"testing"
	"time"
)

func TestExactMatch(t *testing.T) {
	ob := NewOrderBook()

	maker := &Order{
		id: "m1", 
		isBuy: false, 
		quantity: 10, 
		price: 100, 
		timeStamp: time.Now(),
	}
	ob.ProcessOrder(maker)


	taker := &Order{
		id: "t1", 
		isBuy: true, 
		quantity: 10, 
		price: 100, 
		timeStamp: time.Now(),
	}
	ob.ProcessOrder(taker)

	if taker.quantity != 0 {
		t.Errorf("Error: Taker quantity should be 0, got %d", taker.quantity)
	}

	if len(ob.askPrices) != 0 {
		t.Errorf("Error: Price level 100 should have been removed as it is empty")
	}
}





func TestPartialFillMakerLarger(t *testing.T) {
	ob := NewOrderBook()

	maker := &Order{
		id: "m1", 
		isBuy: false, 
		quantity: 10, 
		price: 100, 
		timeStamp: time.Now(),
	}
	ob.ProcessOrder(maker)

	taker := &Order{
		id: "t1",
		isBuy: true, 
		quantity: 4, 
		price: 100, 
		timeStamp: time.Now(),
	}
	ob.ProcessOrder(taker)

	if taker.quantity != 0 {
		t.Errorf("Error: Taker should be fully filled (quantity 0), got %d", taker.quantity)
	}

	limit, exists := ob.Asks[100]
	if !exists {
		t.Fatalf("Error: Limit at 100 should still exist")
	}

	if limit.TotalVolume != 6 {
		t.Errorf("Error: Remaining Maker volume should be 6, got %d", limit.TotalVolume)
	}

}



func TestMassiveTaker(t *testing.T) {
	ob := NewOrderBook()

	ob.ProcessOrder(&Order{
		id: "m1",
		isBuy: false,
		quantity: 5,
		price: 100,
		timeStamp: time.Now(),
	})

	ob.ProcessOrder(&Order{
		id: "m2", 
		isBuy: false, 
		quantity: 5, 
		price: 105, 
		timeStamp: time.Now(),
	})

	taker := &Order{
		id: "t1", 
		isBuy: true, 
		quantity: 15, 
		price: 110, 
		timeStamp: time.Now(),
	}

	ob.ProcessOrder(taker)

	if taker.quantity != 5 {
		t.Errorf("Error: Taker should have consumed 10 shares, 5 should remain, got %d", taker.quantity)
	}

	if len(ob.askPrices) != 0 {
		t.Errorf("Error: The Ask order book should be completely empty, got %d remaining prices", len(ob.askPrices))
	}

	limit, exists := ob.Bids[110]
	if !exists {
		t.Fatalf("Error: Taker should have turned into a Maker at 110")
	}

	if limit.TotalVolume != 5 {
		t.Errorf("Error: New Bid volume should be 5, got %d", limit.TotalVolume)
	}

}