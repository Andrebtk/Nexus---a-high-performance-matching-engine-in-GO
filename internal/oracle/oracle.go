package oracle

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/twelvedata/twelvedata-go/twelvedata"
)



type PriceOracle struct {
	mu       sync.RWMutex
	prices   map[string]float64
	tdClient *twelvedata.APIClient
}


func NewPriceOracle(apiKey string) *PriceOracle {
	cfg, _ := twelvedata.NewConfig(apiKey)
	client := twelvedata.NewAPIClient(cfg)

	return &PriceOracle{
		prices:   make(map[string]float64),
		tdClient: client,
	}
}


func (po *PriceOracle) GetPrice(symbol string) (float64, bool) {
	po.mu.RLock()
	defer po.mu.RUnlock()
	price, exists := po.prices[symbol]
	return price, exists
}


func (po *PriceOracle) SetPrice(symbol string, price float64) {
	po.mu.Lock()
	defer po.mu.Unlock()
	po.prices[symbol] = price
}


func (po *PriceOracle) fetchRealPrice(symbol string) (float64, error) {
	resp, _, err := po.tdClient.MarketDataAPI.GetPrice(context.Background()).
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


func (po *PriceOracle) RunPriceUpdater(symbols []string) {
	ticker := time.NewTicker(1 * time.Minute)
	i := 0

	for _, sym := range symbols {
		if price, err := po.fetchRealPrice(sym); err == nil {
			po.SetPrice(sym, price)
			fmt.Printf("[ORACLE] Initial price set for %s: $%.2f\n", sym, price)
		}
		time.Sleep(2 * time.Second)
	}

	
	for {
		<-ticker.C

		symbol := symbols[i]
		if price, err := po.fetchRealPrice(symbol); err == nil {
			po.SetPrice(symbol, price)
			fmt.Printf("[ORACLE] Updated price for %s: $%.2f\n", symbol, price)
		} else {
			fmt.Printf("[ORACLE ERROR] Failed to update %s: %v\n", symbol, err)
		}

		i = (i + 1) % len(symbols)
	}
}