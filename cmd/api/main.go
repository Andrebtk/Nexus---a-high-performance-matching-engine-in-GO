package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"math/rand"

	"github.com/joho/godotenv"
	"Nexus/internal/api"
	"Nexus/internal/database"
	"Nexus/internal/engine"
	"Nexus/internal/oracle"
	"Nexus/internal/services"
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

func MarketMakerBot(ex *engine.Exchange, symbol string, fallbackPrice uint64, po *oracle.PriceOracle) {
	tradeTicker := time.NewTicker(2 * time.Second)
	botID := "bot_maker_" + symbol

	for {
		<-tradeTicker.C

		// Le bot lit le prix en mémoire locale depuis l'Oracle (0 appel API ici !)
		currentBasePrice := float64(fallbackPrice)
		if p, ok := po.GetPrice(symbol); ok {
			currentBasePrice = p
		}

		baseIntPrice := uint64(currentBasePrice)
		variation := int64(rand.Intn(5)) - 2
		orderPrice := uint64(int64(baseIntPrice) + variation)

		isBuy := rand.Intn(2) == 0

		order := &engine.Order{
			Id:        fmt.Sprintf("%s_%d", botID, time.Now().UnixNano()),
			UserID:    "system_bot",
			Symbol:    symbol,
			IsBuy:     isBuy,
			Quantity:  rand.Intn(15) + 1,
			Price:     orderPrice,
			TimeStamp: time.Now(),
		}
		ex.RouteOrder(order)
	}
}

// loadEnvFile manually loads environment variables from a .env file
func loadEnvFile(filePath string) error {
	log.Printf("Attempting to load .env file from: %s", filePath)
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Failed to open .env file: %v", err)
		return err
	}
	defer file.Close()

	var loadedCount int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip comments and empty lines
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key=value pairs
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, `"'`)
			os.Setenv(key, value)
			loadedCount++
			log.Printf("Loaded env var: %s=%s", key, value)
		}
	}

	log.Printf("Loaded %d environment variables from %s", loadedCount, filePath)
	return scanner.Err()
}

func main() {
	// Load environment variables from .env file
	// Try multiple locations to ensure it works from any working directory
	envFiles := []string{".env", "../.env", "../../.env"}
	var loadedFrom string

	// First try with godotenv
	for _, file := range envFiles {
		err := godotenv.Load(file)
		if err == nil {
			loadedFrom = file
			break
		}
	}

	// If godotenv failed, try manual loading
	if loadedFrom == "" {
		for _, file := range envFiles {
			err := loadEnvFile(file)
			if err == nil {
				loadedFrom = file
				break
			}
		}
	}

	if loadedFrom != "" {
		log.Printf("Successfully loaded .env file from: %s", loadedFrom)
	} else {
		log.Println("Warning: No .env file found, using environment variables")
	}

	fmt.Println("Starting Nexus matching engine...")

	// Initialize PostgreSQL database
	db, err := database.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	// Create PostgreSQL user service
	postgresUserService := services.NewPostgresUserService(db)

	// Create system bot if it doesn't exist
	err = postgresUserService.CreateSystemBotIfNotExists()
	if err != nil {
		log.Printf("Warning: Failed to create system bot: %v", err)
	}

	// Get system bot user (for reference, not currently used)
	_, err = postgresUserService.GetUserByEmail("system@nexus.com")
	if err != nil {
		log.Fatalf("Failed to get system bot: %v", err)
	}

	// Create in-memory user service for existing functionality
	userService := services.NewUserService()
	userService.CreateUser("system_bot")

	botUser, err := userService.GetUser("system_bot")
	if err == nil {
		botUser.Balance = 10000000
	}

	transactionService := services.NewPostgresTransactionService(database.DB)
	costBasisService := services.NewCostBasisService(database.DB)
	profitLossService := services.NewProfitLossService(userService, postgresUserService, transactionService)
	orderService := services.NewOrderService(database.DB)

	ex := engine.NewExchange(userService, transactionService, profitLossService, orderService, postgresUserService, costBasisService)
	//populate(ex)

	fmt.Println("Starting Price Oracle...")
	twelveDataAPIKey := os.Getenv("TWELVEDATA_API_KEY")
	if twelveDataAPIKey == "" {
		log.Fatal("TWELVEDATA_API_KEY environment variable is not set")
	}

	po := oracle.NewPriceOracle(twelveDataAPIKey)
	symbols := []string{"AAPL", "MSFT", "NVDA", "TSLA"}

	go po.RunPriceUpdater(symbols)

	fmt.Println("Waking up Market Makers (bots)...")
	go MarketMakerBot(ex, "AAPL", 150, po)
	go MarketMakerBot(ex, "MSFT", 400, po)
	go MarketMakerBot(ex, "NVDA", 120, po)
	go MarketMakerBot(ex, "TSLA", 200, po)

	// Set JWT secret after environment variables are loaded
	api.SetJWTSecret(os.Getenv("JWT_SECRET"))

	api.StartAPI(ex, profitLossService, postgresUserService)
}