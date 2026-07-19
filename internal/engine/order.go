package engine 


import "time"


type Order struct {
	Id        string
	Symbol    string
	IsBuy     bool
	Quantity  int
	Price     uint64
	TimeStamp time.Time

	next *Order
	prev *Order
}
