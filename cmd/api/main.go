package main

import (
	"context"
	"fmt"
	"time"
	"math/rand"
	"strconv"

	"Nexus/internal/api"
	"Nexus/internal/engine"

	"github.com/twelvedata/twelvedata-go/twelvedata"
)


var tdClient *twelvedata.APIClient

func initTwelveData(apiKey string) {
	cfg, _ := twelvedata.NewConfig(apiKey)
	tdClient = twelvedata.NewAPIClient(cfg)
}


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

func populate(ex *engine.Exchange) {
	fmt.Println("Initial populating of the Exchange in progress...")

	initialOrders := []*engine.Order{
		// 150$
		{Id: "pop_aapl_ask1", Symbol: "AAPL", IsBuy: false, Quantity: 50, Price: 151, TimeStamp: time.Now()},
		{Id: "pop_aapl_ask2", Symbol: "AAPL", IsBuy: false, Quantity: 120, Price: 152, TimeStamp: time.Now()},
		{Id: "pop_aapl_bid1", Symbol: "AAPL", IsBuy: true, Quantity: 200, Price: 149, TimeStamp: time.Now()},
		{Id: "pop_aapl_bid2", Symbol: "AAPL", IsBuy: true, Quantity: 80, Price: 148, TimeStamp: time.Now()},

		// 400$
		{Id: "pop_msft_ask1", Symbol: "MSFT", IsBuy: false, Quantity: 30, Price: 401, TimeStamp: time.Now()},
		{Id: "pop_msft_ask2", Symbol: "MSFT", IsBuy: false, Quantity: 10, Price: 405, TimeStamp: time.Now()},
		{Id: "pop_msft_bid1", Symbol: "MSFT", IsBuy: true, Quantity: 45, Price: 398, TimeStamp: time.Now()},
		{Id: "pop_msft_bid2", Symbol: "MSFT", IsBuy: true, Quantity: 100, Price: 395, TimeStamp: time.Now()},

		// 120$
		{Id: "pop_nvda_ask1", Symbol: "NVDA", IsBuy: false, Quantity: 500, Price: 121, TimeStamp: time.Now()},
		{Id: "pop_nvda_ask2", Symbol: "NVDA", IsBuy: false, Quantity: 250, Price: 122, TimeStamp: time.Now()},
		{Id: "pop_nvda_bid1", Symbol: "NVDA", IsBuy: true, Quantity: 300, Price: 119, TimeStamp: time.Now()},
		{Id: "pop_nvda_bid2", Symbol: "NVDA", IsBuy: true, Quantity: 600, Price: 118, TimeStamp: time.Now()},

		// 200$
		{Id: "pop_tsla_ask1", Symbol: "TSLA", IsBuy: false, Quantity: 15, Price: 202, TimeStamp: time.Now()},
		{Id: "pop_tsla_ask2", Symbol: "TSLA", IsBuy: false, Quantity: 40, Price: 205, TimeStamp: time.Now()},
		{Id: "pop_tsla_bid1", Symbol: "TSLA", IsBuy: true, Quantity: 20, Price: 198, TimeStamp: time.Now()},
		{Id: "pop_tsla_bid2", Symbol: "TSLA", IsBuy: true, Quantity: 50, Price: 195, TimeStamp: time.Now()},
	}


	for _, order := range initialOrders {
		ex.RouteOrder(order)
	}


	fmt.Println("✅ AAPL, MSFT, NVDA, and TSLA markets successfully initialized !")
}

func fetchRealPrice(symbol string) (float64, error) {
	resp, _, err := tdClient.MarketDataAPI.GetPrice(context.Background()).
		Symbol(symbol).
		Execute()
	
	if err != nil {
		return 0, err
	}

	price, err := strconv.ParseFloat(resp.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("could not parse price: %v", err)
	}

	return price, nil
}



func MarketMakerBot(ex *engine.Exchange, symbol string, fallbackPrice uint64) {
	tradeTicker := time.NewTicker(2 * time.Second)
	priceTicker := time.NewTicker(30 * time.Second)

	botID := "bot_maker_" + symbol
	currentBasePrice := float64(fallbackPrice)

	if price, err := fetchRealPrice(symbol); err == nil {
		currentBasePrice = price
		fmt.Printf("[INFO] Real price fetched for %s: $%.2f\n", symbol, currentBasePrice)
	} else {
		fmt.Printf("[WARN] %v. Using default price: $%d\n", err, fallbackPrice)
	}



	for {
		select {
			case <-priceTicker.C:
				// Every 30 seconds, update the base price from the API
				if price, err := fetchRealPrice(symbol); err == nil {
					currentBasePrice = price
				}
			
			case <-tradeTicker.C:
				// Every 2 seconds, place a new order around the currentBasePrice
				baseIntPrice := uint64(currentBasePrice)
				
				variation := int64(rand.Intn(5)) - 2
				orderPrice := uint64(int64(baseIntPrice) + variation)

				isBuy := rand.Intn(2) == 0

				order := &engine.Order{
					Id:        fmt.Sprintf("%s_%d", botID, time.Now().UnixNano()),
					Symbol:    symbol,
					IsBuy:     isBuy,
					Quantity:  rand.Intn(15) + 1,
					Price:     orderPrice,
					TimeStamp: time.Now(),
				}
				ex.RouteOrder(order)

		}
	}
}



func main() {
	fmt.Println("Starting Nexus matching engine...")
	initTwelveData("081f90e89a2447a48c79296b458cfd98")
	ex := engine.NewExchange()
	populate(ex)

	fmt.Println("Waking up Market Makers (bots) and connecting to Yahoo Finance...")
	go MarketMakerBot(ex, "AAPL", 150)
	go MarketMakerBot(ex, "MSFT", 400)
	go MarketMakerBot(ex, "NVDA", 120)
	go MarketMakerBot(ex, "TSLA", 200)
	
	api.StartAPI(ex)
}