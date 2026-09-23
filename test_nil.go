package main

import (
	"fmt"
	"time"
	"Nexus/internal/engine"
)

func main() {
	ex := engine.NewExchange(nil, nil, nil, nil, nil, nil)
	o1 := &engine.Order{
		Id: "1", UserID: "999", Symbol: "AAPL", IsBuy: false, Quantity: 10, Price: 100, TimeStamp: time.Now(),
	}
	o2 := &engine.Order{
		Id: "2", UserID: "999", Symbol: "AAPL", IsBuy: true, Quantity: 10, Price: 105, TimeStamp: time.Now(),
	}
	
	ex.RouteOrder(o1)
	ex.RouteOrder(o2)
	
	time.Sleep(1 * time.Second)
	fmt.Println("Done")
}
