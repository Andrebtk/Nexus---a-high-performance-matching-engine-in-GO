package main

import (
	"fmt"
	"time"
	"sort"
	"sync"
)

type Order struct {
	id        string
	isBuy     bool
	quantity  int
	price     uint64
	timeStamp time.Time

	next *Order
	prev *Order
}

type OrderQueue struct {
	head *Order
	tail *Order
}

func (oq *OrderQueue) Add(o *Order) {
	if o == nil {
		return
	}

	if oq.tail == nil {
		oq.tail = o
		oq.head = o
	} else {
		o.prev = oq.tail
		oq.tail.next = o
		oq.tail = o
	}
}

func (oq *OrderQueue) Remove(o *Order) {

	if o == nil {
		return
	}


	if o == oq.head && o == oq.tail {
		oq.head = nil
		oq.tail = nil

	} else if oq.head == o {
		oq.head = o.next
		oq.head.prev = nil

	} else if oq.tail == o {
		oq.tail = o.prev
		oq.tail.next = nil

	} else {
		o.prev.next = o.next
		o.next.prev = o.prev

		o.prev = nil
		o.next = nil
	}

}

func (oq *OrderQueue) Pop() *Order {
	if oq.head == nil {
		return nil
	}

	result := oq.head

	if oq.head == oq.tail {
		oq.head = nil
		oq.tail = nil
	} else {
		oq.head.next.prev = nil
		oq.head = oq.head.next
	}

	result.next = nil
	result.prev = nil

	return result
}


func (oq *OrderQueue) IsEmpty() bool {
	return oq.head == nil
}



type Limit struct {
	price uint64
	doubleLinkedList OrderQueue
	totalVolume int 
}

func NewLimit(price uint64) *Limit {
	return &Limit{
		price:       price,
		totalVolume: 0,
		doubleLinkedList: OrderQueue{
			head: nil,
			tail: nil,
		},
	}
}

func (l *Limit) AddOrder(o *Order) {
	l.doubleLinkedList.Add(o)
	l.totalVolume += o.quantity
}

func (l *Limit) CancelOrder(o *Order) {
	l.doubleLinkedList.Remove(o)
	l.totalVolume -= o.quantity
}

func (l *Limit) Pop() *Order {
	order := l.doubleLinkedList.Pop()

	if order != nil {
		l.totalVolume -= order.quantity
	}

	return order
}



type OrderBook struct {
	Bids map[uint64]*Limit
	Asks map[uint64]*Limit

	orders map[string]*Order

	bidPrices []uint64
	askPrices []uint64

	mu sync.RWMutex
}


func NewOrderBook() *OrderBook {
	return &OrderBook {
		Bids: make(map[uint64]*Limit),
		Asks: make(map[uint64]*Limit),
		orders: make(map[string]*Order),

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



func (ob *OrderBook) matchBuy(o *Order) {
	for o.quantity > 0 && len(ob.askPrices) > 0 {
		bestAsk := ob.askPrices[0]
		
		if o.price < bestAsk {
			return 
		} 


		limit := ob.Asks[bestAsk]

		for o.quantity >0 && !limit.doubleLinkedList.IsEmpty() {
			tmp := limit.doubleLinkedList.head 

			if o.quantity >= tmp.quantity {

				remaining := tmp.quantity
				limit.Pop()
				o.quantity -= remaining
			} else {
				tmp.quantity -= o.quantity
				limit.totalVolume -= o.quantity
				o.quantity = 0
			}
		}


		if limit.doubleLinkedList.IsEmpty() {
			delete(ob.Asks, bestAsk)
			ob.askPrices = ob.askPrices[1:]
		}
		
	}
}

func (ob *OrderBook) matchSell(o *Order) {
	for o.quantity > 0 && len(ob.bidPrices) > 0 {
		bestBid := ob.bidPrices[0]
		if o.price > bestBid {
			return 
		}

		limit := ob.Bids[bestBid]

		for o.quantity > 0 && !limit.doubleLinkedList.IsEmpty() {
			tmp := limit.doubleLinkedList.head 

			if o.quantity >= tmp.quantity {

				remaining := tmp.quantity
				limit.Pop()
				o.quantity -= remaining
			} else {
				tmp.quantity -= o.quantity
				limit.totalVolume -= o.quantity
				o.quantity = 0
			}
		}

		if limit.doubleLinkedList.IsEmpty() {
			delete(ob.Bids, bestBid)
			ob.bidPrices = ob.bidPrices[1:]
		}
	}
}

func (ob *OrderBook) placeMakerOrder(o *Order) {

	var targetMap map[uint64]*Limit
	if o.isBuy {
		targetMap = ob.Bids
	} else {
		targetMap = ob.Asks
	}

	limit, ok := targetMap[o.price]

	if !ok {
		limit = NewLimit(o.price)
		targetMap[o.price] = limit
		ob.addPrice(o.price, o.isBuy)
	}


	limit.AddOrder(o)

	ob.orders[o.id] = o
}



func (ob *OrderBook) ProcessOrder(o *Order) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if o.isBuy {
		ob.matchBuy(o)
	} else {
		ob.matchSell(o)
	}


	if o.quantity > 0 {
		ob.placeMakerOrder(o)
	}
}




func main() {
	ob := NewOrderBook()

	// 1. On place un ordre de VENTE (Maker) de 10 actions à 100$
	sellOrder := &Order{
		id:        "order_1",
		isBuy:     false,
		quantity:  10,
		price:     100,
		timeStamp: time.Now(),
	}
	ob.ProcessOrder(sellOrder)
	fmt.Printf("Après placement Vendeur : %d vendeurs au prix de 100$\n", ob.Asks[100].totalVolume)

	// 2. On place un ordre d'ACHAT (Taker) de 4 actions à 100$
	buyOrder := &Order{
		id:        "order_2",
		isBuy:     true,
		quantity:  4,
		price:     100,
		timeStamp: time.Now(),
	}
	ob.ProcessOrder(buyOrder)

	// 3. On vérifie le résultat
	fmt.Printf("Après matching Acheteur : Le vendeur n'a plus que %d actions à vendre.\n", ob.Asks[100].totalVolume)
	fmt.Printf("L'ordre d'achat a été consommé, quantité restante : %d\n", buyOrder.quantity)
}