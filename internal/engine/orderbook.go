package engine

import (
	"sort"
	"sync"
)

// Fill represents a matched portion of an order
type Fill struct {
	MakerOrder *Order
	Quantity   int
	Price      uint64
}



type OrderBook struct {
	Bids map[uint64]*Limit
	Asks map[uint64]*Limit

	orders       map[string]*Order
	ordersByDBID map[int]*Order // NEW: lookup for cancellation by Postgres order ID

	bidPrices []uint64
	askPrices []uint64

	mu sync.RWMutex
}


func NewOrderBook() *OrderBook {
	return &OrderBook {
		Bids: make(map[uint64]*Limit),
		Asks: make(map[uint64]*Limit),
		orders: make(map[string]*Order),
		ordersByDBID: make(map[int]*Order),

		bidPrices: []uint64{},
		askPrices: []uint64{},
	}
}

func (ob *OrderBook) addPrice(price uint64, isBuy bool) {
	if isBuy {
		// Bids: Tri décroissant (100, 90, 80...)
		// On cherche le premier index où la valeur est <= au prix (pour insérer juste avant)
		idx := sort.Search(len(ob.bidPrices), func(i int) bool {
			return ob.bidPrices[i] <= price
		})

		// Vérifier si le prix existe déjà
		if idx < len(ob.bidPrices) && ob.bidPrices[idx] == price {
			return
		}

		// Insérer au bon endroit
		ob.bidPrices = append(ob.bidPrices, 0)
		copy(ob.bidPrices[idx+1:], ob.bidPrices[idx:])
		ob.bidPrices[idx] = price

	} else {
		// Asks: Tri croissant (80, 90, 100...)
		// On cherche le premier index où la valeur est >= au prix
		idx := sort.Search(len(ob.askPrices), func(i int) bool {
			return ob.askPrices[i] >= price
		})

		if idx < len(ob.askPrices) && ob.askPrices[idx] == price {
			return
		}

		ob.askPrices = append(ob.askPrices, 0)
		copy(ob.askPrices[idx+1:], ob.askPrices[idx:])
		ob.askPrices[idx] = price
	}
}

func (ob *OrderBook) RemovePrice(price uint64, isBuy bool) {
	if isBuy {
		for i, p := range ob.bidPrices {
			if p == price {
				// Suppression : on retire l'élément en shiftant les autres
				ob.bidPrices = append(ob.bidPrices[:i], ob.bidPrices[i+1:]...)
				break
			}
		}
	} else {
		for i, p := range ob.askPrices {
			if p == price {
				ob.askPrices = append(ob.askPrices[:i], ob.askPrices[i+1:]...)
				break
			}
		}
	}
}



func (ob *OrderBook) matchBuy(o *Order) []Fill {
	var fills []Fill
	for o.Quantity > 0 && len(ob.askPrices) > 0 {
		bestAsk := ob.askPrices[0]

		if o.Price < bestAsk {
			return fills
		}

		limit := ob.Asks[bestAsk]

		for o.Quantity > 0 && !limit.doubleLinkedList.IsEmpty() {
			tmp := limit.doubleLinkedList.head

			if o.Quantity >= tmp.Quantity {
				matchedQty := tmp.Quantity
				limit.Pop()
				o.Quantity -= matchedQty
				tmp.Quantity = 0 // FIX: Set maker quantity to 0 when fully matched
				delete(ob.orders, tmp.Id)
				if tmp.DBOrderID > 0 {
					delete(ob.ordersByDBID, tmp.DBOrderID)
				}
				fills = append(fills, Fill{
					MakerOrder: tmp,
					Quantity:   matchedQty,
					Price:      bestAsk,
				})
			} else {
				matchedQty := o.Quantity
				tmp.Quantity -= matchedQty
				limit.TotalVolume -= matchedQty
				o.Quantity = 0
				fills = append(fills, Fill{
					MakerOrder: tmp,
					Quantity:   matchedQty,
					Price:      bestAsk,
				})
			}
		}

		if limit.doubleLinkedList.IsEmpty() {
			delete(ob.Asks, bestAsk)
			ob.askPrices = ob.askPrices[1:]
		}
	}
	return fills
}

func (ob *OrderBook) matchSell(o *Order) []Fill {
	var fills []Fill
	for o.Quantity > 0 && len(ob.bidPrices) > 0 {
		bestBid := ob.bidPrices[0]
		// Sell order matches if sell price <= best bid price (seller accepts bidder's price)
		if o.Price > bestBid {
			return fills
		}

		limit := ob.Bids[bestBid]

		for o.Quantity > 0 && !limit.doubleLinkedList.IsEmpty() {
			tmp := limit.doubleLinkedList.head

			if o.Quantity >= tmp.Quantity {
				matchedQty := tmp.Quantity
				limit.Pop()
				o.Quantity -= matchedQty
				tmp.Quantity = 0 // FIX: Set maker quantity to 0 when fully matched
				delete(ob.orders, tmp.Id)
				if tmp.DBOrderID > 0 {
					delete(ob.ordersByDBID, tmp.DBOrderID)
				}
				fills = append(fills, Fill{
					MakerOrder: tmp,
					Quantity:   matchedQty,
					Price:      bestBid,
				})
			} else {
				matchedQty := o.Quantity
				tmp.Quantity -= matchedQty
				limit.TotalVolume -= matchedQty
				o.Quantity = 0
				fills = append(fills, Fill{
					MakerOrder: tmp,
					Quantity:   matchedQty,
					Price:      bestBid,
				})
			}
		}

		if limit.doubleLinkedList.IsEmpty() {
			delete(ob.Bids, bestBid)
			ob.bidPrices = ob.bidPrices[1:]
		}
	}
	return fills
}

func (ob *OrderBook) placeMakerOrder(o *Order) {

	var targetMap map[uint64]*Limit

	if o.IsBuy {
		targetMap = ob.Bids
	} else {
		targetMap = ob.Asks
	}

	limit, ok := targetMap[o.Price]

	if !ok {
		limit = NewLimit(o.Price)
		targetMap[o.Price] = limit
		ob.addPrice(o.Price, o.IsBuy)
	}


	limit.AddOrder(o)

	ob.orders[o.Id] = o
	if o.DBOrderID > 0 {
		ob.ordersByDBID[o.DBOrderID] = o
	}
}



func (ob *OrderBook) ProcessOrder(o *Order) []Fill {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	var fills []Fill

	if o.IsBuy {
		fills = ob.matchBuy(o)
	} else {
		fills = ob.matchSell(o)
	}

	if o.Quantity > 0 {
		ob.placeMakerOrder(o)
	}

	return fills
}

// CancelOrder removes a resting (unfilled) order from the book by its
// Postgres order ID. Returns the removed Order, or nil if it wasn't
// found resting in the book (e.g. already filled or never placed here).
func (ob *OrderBook) CancelOrder(dbOrderID int) *Order {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	order, exists := ob.ordersByDBID[dbOrderID]
	if !exists {
		return nil
	}

	var targetMap map[uint64]*Limit
	if order.IsBuy {
		targetMap = ob.Bids
	} else {
		targetMap = ob.Asks
	}

	limit, ok := targetMap[order.Price]
	if ok {
		limit.CancelOrder(order)
		if limit.doubleLinkedList.IsEmpty() {
			delete(targetMap, order.Price)
			ob.RemovePrice(order.Price, order.IsBuy)
		}
	}

	delete(ob.orders, order.Id)
	delete(ob.ordersByDBID, dbOrderID)

	return order
}
