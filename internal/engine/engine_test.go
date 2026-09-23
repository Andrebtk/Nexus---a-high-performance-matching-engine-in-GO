package engine

import (
	"testing"
	"time"
)

func TestExactMatch(t *testing.T) {
	ob := NewOrderBook()

	maker := &Order{
		Id:        "m1",
		IsBuy:     false,
		Quantity:  10,
		Price:     100,
		TimeStamp: time.Now(),
	}
	ob.ProcessOrder(maker)

	taker := &Order{
		Id:        "t1",
		IsBuy:     true,
		Quantity:  10,
		Price:     100,
		TimeStamp: time.Now(),
	}
	ob.ProcessOrder(taker)

	if taker.Quantity != 0 {
		t.Errorf("Error: Taker quantity should be 0, got %d", taker.Quantity)
	}

	if len(ob.askPrices) != 0 {
		t.Errorf("Error: Price level 100 should have been removed as it is empty")
	}
}

func TestPartialFillMakerLarger(t *testing.T) {
	ob := NewOrderBook()

	maker := &Order{
		Id:        "m1",
		IsBuy:     false,
		Quantity:  10,
		Price:     100,
		TimeStamp: time.Now(),
	}
	ob.ProcessOrder(maker)

	taker := &Order{
		Id:        "t1",
		IsBuy:     true,
		Quantity:  4,
		Price:     100,
		TimeStamp: time.Now(),
	}
	ob.ProcessOrder(taker)

	if taker.Quantity != 0 {
		t.Errorf("Error: Taker should be fully filled (quantity 0), got %d", taker.Quantity)
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
		Id:        "m1",
		IsBuy:     false,
		Quantity:  5,
		Price:     100,
		TimeStamp: time.Now(),
	})

	ob.ProcessOrder(&Order{
		Id:        "m2",
		IsBuy:     false,
		Quantity:  5,
		Price:     105,
		TimeStamp: time.Now(),
	})

	taker := &Order{
		Id:        "t1",
		IsBuy:     true,
		Quantity:  15,
		Price:     110,
		TimeStamp: time.Now(),
	}

	ob.ProcessOrder(taker)

	if taker.Quantity != 5 {
		t.Errorf("Error: Taker should have consumed 10 shares, 5 should remain, got %d", taker.Quantity)
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

func TestFIFOPriority(t *testing.T) {
	ob := NewOrderBook()

	// Two maker sell orders at the exact same price
	m1 := &Order{
		Id:        "m1",
		IsBuy:     false,
		Quantity:  5,
		Price:     100,
		TimeStamp: time.Now(),
	}
	ob.ProcessOrder(m1)

	time.Sleep(1 * time.Millisecond)

	m2 := &Order{
		Id:        "m2",
		IsBuy:     false,
		Quantity:  5,
		Price:     100,
		TimeStamp: time.Now(),
	}
	ob.ProcessOrder(m2)

	// Taker buys 6 shares: should fully fill m1 (5 shares) and take 1 share from m2
	taker := &Order{
		Id:        "t1",
		IsBuy:     true,
		Quantity:  6,
		Price:     100,
		TimeStamp: time.Now(),
	}
	fills := ob.ProcessOrder(taker)

	if taker.Quantity != 0 {
		t.Errorf("Expected taker to be completely filled, remaining: %d", taker.Quantity)
	}

	if len(fills) != 2 {
		t.Fatalf("Expected 2 fills, got %d", len(fills))
	}

	if fills[0].MakerOrder.Id != "m1" || fills[0].Quantity != 5 {
		t.Errorf("Expected first fill to be m1 for 5 shares, got id=%s qty=%d", fills[0].MakerOrder.Id, fills[0].Quantity)
	}

	if fills[1].MakerOrder.Id != "m2" || fills[1].Quantity != 1 {
		t.Errorf("Expected second fill to be m2 for 1 share, got id=%s qty=%d", fills[1].MakerOrder.Id, fills[1].Quantity)
	}

	limit, exists := ob.Asks[100]
	if !exists {
		t.Fatalf("Limit at 100 should still exist")
	}
	if limit.TotalVolume != 4 {
		t.Errorf("Expected remaining volume at 100 to be 4, got %d", limit.TotalVolume)
	}
}

func TestSellTakerMatchesBuyMaker(t *testing.T) {
	ob := NewOrderBook()

	// Maker places a Buy order at 100
	maker := &Order{
		Id:        "m_buy",
		IsBuy:     true,
		Quantity:  10,
		Price:     100,
		TimeStamp: time.Now(),
	}
	ob.ProcessOrder(maker)

	// Taker places a Sell order at 95 (willing to sell for <= 100)
	taker := &Order{
		Id:        "t_sell",
		IsBuy:     false,
		Quantity:  6,
		Price:     95,
		TimeStamp: time.Now(),
	}
	fills := ob.ProcessOrder(taker)

	if taker.Quantity != 0 {
		t.Errorf("Expected taker sell to be fully filled, remaining: %d", taker.Quantity)
	}

	if len(fills) != 1 {
		t.Fatalf("Expected 1 fill, got %d", len(fills))
	}

	// The execution price should be the maker's price (100)
	if fills[0].Price != 100 {
		t.Errorf("Expected fill price to be 100 (maker price), got %d", fills[0].Price)
	}

	limit, exists := ob.Bids[100]
	if !exists {
		t.Fatalf("Limit 100 should still exist on Bids")
	}
	if limit.TotalVolume != 4 {
		t.Errorf("Expected remaining Bid volume to be 4, got %d", limit.TotalVolume)
	}
}

func TestNoMatchCrossSpread(t *testing.T) {
	ob := NewOrderBook()

	// Maker asks 105
	ask := &Order{
		Id:        "m_ask",
		IsBuy:     false,
		Quantity:  10,
		Price:     105,
		TimeStamp: time.Now(),
	}
	ob.ProcessOrder(ask)

	// Buyer bids 100 (bid < ask, so no match)
	bid := &Order{
		Id:        "m_bid",
		IsBuy:     true,
		Quantity:  5,
		Price:     100,
		TimeStamp: time.Now(),
	}
	fills := ob.ProcessOrder(bid)

	if len(fills) != 0 {
		t.Errorf("Expected 0 fills when spread is not crossed, got %d", len(fills))
	}
	if bid.Quantity != 5 {
		t.Errorf("Expected bid quantity to remain 5, got %d", bid.Quantity)
	}

	if len(ob.bidPrices) != 1 || ob.bidPrices[0] != 100 {
		t.Errorf("Expected bid price level 100 to be recorded")
	}
	if len(ob.askPrices) != 1 || ob.askPrices[0] != 105 {
		t.Errorf("Expected ask price level 105 to be recorded")
	}
}

func TestCancelOrder(t *testing.T) {
	ob := NewOrderBook()

	order := &Order{
		Id:        "o_cancel",
		DBOrderID: 42,
		IsBuy:     true,
		Quantity:  10,
		Price:     100,
		TimeStamp: time.Now(),
	}
	ob.ProcessOrder(order)

	if _, exists := ob.Bids[100]; !exists {
		t.Fatalf("Expected order to be placed in Bids[100]")
	}

	cancelled := ob.CancelOrder(42)
	if cancelled == nil {
		t.Fatalf("Expected CancelOrder to return the cancelled order")
	}

	if _, exists := ob.Bids[100]; exists {
		t.Errorf("Expected price level 100 to be deleted after order cancellation")
	}

	if len(ob.bidPrices) != 0 {
		t.Errorf("Expected bidPrices to be empty after cancelling the only order")
	}

	// Cancelling again should return nil
	if secondCancel := ob.CancelOrder(42); secondCancel != nil {
		t.Errorf("Expected second cancellation to return nil")
	}
}