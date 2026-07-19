package main

import (
	"fmt"
	"time"

	//"Nexus/internal/api"
	"Nexus/internal/engine"
)

func testOrderBook() {
	ob := engine.NewOrderBook()

	// 1. On place un ordre de VENTE (Maker) de 10 actions à 100$
	sellOrder := &engine.Order{
		Id:        "order_1",
		IsBuy:     false,
		Quantity:  10,
		Price:     100,
		TimeStamp: time.Now(),
	}
	ob.ProcessOrder(sellOrder)
	fmt.Printf("Après placement Vendeur : %d vendeurs au prix de 100$\n", ob.Asks[100].TotalVolume)

	// 2. On place un ordre d'ACHAT (Taker) de 4 actions à 100$
	buyOrder := &engine.Order{
		Id:        "order_2",
		IsBuy:     true,
		Quantity:  4,
		Price:     100,
		TimeStamp: time.Now(),
	}
	ob.ProcessOrder(buyOrder)

	// 3. On vérifie le résultat
	fmt.Printf("Après matching Acheteur : Le vendeur n'a plus que %d actions à vendre.\n", ob.Asks[100].TotalVolume)
	fmt.Printf("L'ordre d'achat a été consommé, quantité restante : %d\n", buyOrder.Quantity)
}

func main() {
	testOrderBook()
}