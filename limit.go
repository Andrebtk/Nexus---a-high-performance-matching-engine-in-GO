package main 




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