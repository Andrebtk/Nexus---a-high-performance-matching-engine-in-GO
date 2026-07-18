package main

import (
	"fmt"
	"time"
)









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