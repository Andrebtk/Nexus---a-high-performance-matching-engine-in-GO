package main 


import "time"


type Order struct {
	id        string
	isBuy     bool
	quantity  int
	price     uint64
	timeStamp time.Time

	next *Order
	prev *Order
}
