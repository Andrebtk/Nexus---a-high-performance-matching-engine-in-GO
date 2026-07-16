package main

import (
	"fmt"
	"time"
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



type Limit struct {
	price uint64
	doubleLinkedList OrderQueue
	totalVolume int 
}

func (l *Limit) AddOrder(o *Order) {
	l.doubleLinkedList.Add(o)
	l.totalVolume += o.quantity
}

func (l *Limit) CancelOrder(o *Order) {
	l.doubleLinkedList.Remove(o)
	l.totalVolume -= o.quantity
}



type OrderBook struct {
	Bids map[uint64]*Limit
	Asks map[uint64]*Limit

	orders map[string]*Order

	bestBids []uint64
	bestAsks []uint64
}




func main() {
	fmt.Println("Hello there")
}
