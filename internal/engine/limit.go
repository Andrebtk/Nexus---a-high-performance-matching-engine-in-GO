package engine 




type Limit struct {
	Price uint64
	doubleLinkedList OrderQueue
	TotalVolume int 
}

func NewLimit(price uint64) *Limit {
	return &Limit{
		Price:       price,
		TotalVolume: 0,
		doubleLinkedList: OrderQueue{
			head: nil,
			tail: nil,
		},
	}
}

func (l *Limit) AddOrder(o *Order) {
	l.doubleLinkedList.Add(o)
	l.TotalVolume += o.Quantity
}

func (l *Limit) CancelOrder(o *Order) {
	l.doubleLinkedList.Remove(o)
	l.TotalVolume -= o.Quantity
}

func (l *Limit) Pop() *Order {
	order := l.doubleLinkedList.Pop()

	if order != nil {
		l.TotalVolume -= order.Quantity
	}

	return order
}