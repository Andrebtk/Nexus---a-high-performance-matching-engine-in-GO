package api 

import (
	"fmt"
	"Nexus/internal/database"
	"Nexus/internal/engine"
	"Nexus/internal/services"
	"sort"
	"net/http"
	"strconv"
	"time"
	"log"
	"github.com/gin-gonic/gin"
)



func TestingHttp(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "testing",
	})
}


func GetExchangeTickets(ex *engine.Exchange) gin.HandlerFunc {
	return func(c *gin.Context) {
		tickers := ex.GetTickers()

		c.JSON(http.StatusOK, gin.H {
			"total": len(tickers),
			"tickers": tickers,
		})
	}
}

func GetOrderBookHandler(ex *engine.Exchange) gin.HandlerFunc {
    return func(c *gin.Context) {

        symbol := c.Query("symbol")
        if symbol == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Missing symbol parameter"})
            return
        }

        ob := ex.GetOrderBook(symbol)
        if ob == nil {
            c.JSON(http.StatusOK, gin.H{
                "bids": []interface{}{},
                "asks": []interface{}{},
                "spread": 0,
            })
            return
        }

        type PriceLevel struct {
            Price    float64 `json:"price"`
            Quantity int     `json:"quantity"`
        }

        // FIX 1 : Initialisation stricte pour éviter que l'API renvoie "null" en JSON
        bids := []PriceLevel{}
        asks := []PriceLevel{}

        for price, limit := range ob.Bids {
            bids = append(bids, PriceLevel{
                Price:    float64(price),
                Quantity: int(limit.TotalVolume),
            })
        }
        sort.Slice(bids, func(i, j int) bool { return bids[i].Price > bids[j].Price })

        for price, limit := range ob.Asks {
            asks = append(asks, PriceLevel{
                Price:    float64(price),
                Quantity: int(limit.TotalVolume),
            })
        }
        sort.Slice(asks, func(i, j int) bool { return asks[i].Price < asks[j].Price })

        // FIX 2 : Renvoyer la réponse finale au client (ça manquait !)
        c.JSON(http.StatusOK, gin.H{
            "bids": bids,
            "asks": asks,
            "spread": 0, // Tu pourras rajouter la vraie logique du spread ici plus tard
        })
    }
}

func GetProfitLossHandler(pls *services.ProfitLossService, postgresUserService *services.PostgresUserService) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.Query("user_id")
        if userID == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user_id parameter"})
            return
        }

        // Check if this is a PostgreSQL user ID (numeric) or in-memory user ID (string)
        var profit, loss float64
        var err error

        // If userID is numeric, it's a PostgreSQL user
        if _, err := strconv.Atoi(userID); err == nil {
            // Get profit/loss from PostgreSQL user
            userIDInt, _ := strconv.Atoi(userID)
            user, err := postgresUserService.GetUserByID(userIDInt)
            if err == nil {
                profit = user.Profit
                loss = user.Loss
            }
        } else {
            // Get profit/loss from in-memory system (for system_bot)
            profit, loss, err = pls.GetUserProfitLoss(userID)
        }

        if err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
            return
        }

        c.JSON(http.StatusOK, gin.H{
            "user_id": userID,
            "profit": profit,
            "loss": loss,
            "net": profit + loss,
        })
    }
}

func CalculateProfitLossHandler(pls *services.ProfitLossService) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.Query("user_id")
        if userID == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user_id parameter"})
            return
        }

        profit, loss, err := pls.CalculateProfitLoss(userID)
        if err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
            return
        }

        c.JSON(http.StatusOK, gin.H{
            "user_id": userID,
            "profit": profit,
            "loss": loss,
            "net": profit + loss,
            "message": "Profit and loss calculated successfully",
        })
    }
}

func GetActiveOrdersHandler(orderService *services.OrderService) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.Query("user_id")
        if userID == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user_id parameter"})
            return
        }

        // Convert userID to integer
        userIDInt, err := strconv.Atoi(userID)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
            return
        }

        orders, err := orderService.GetActiveOrders(userIDInt)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch active orders"})
            return
        }

        log.Printf("INFO: Fetched %d active orders for user %d", len(orders), userIDInt)

        c.JSON(http.StatusOK, gin.H{
            "user_id": userID,
            "active_orders": orders,
        })
    }
}

func GetOrderHistoryHandler(orderService *services.OrderService) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.Query("user_id")
        if userID == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user_id parameter"})
            return
        }

        // Convert userID to integer
        userIDInt, err := strconv.Atoi(userID)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
            return
        }

        orders, err := orderService.GetOrderHistory(userIDInt)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch order history"})
            return
        }

        log.Printf("INFO: Fetched %d historical orders for user %d", len(orders), userIDInt)

        c.JSON(http.StatusOK, gin.H{
            "user_id": userID,
            "order_history": orders,
        })
    }
}

func PlaceOrderHandler(ex *engine.Exchange, postgresUserService *services.PostgresUserService, orderService *services.OrderService) gin.HandlerFunc {
    return func(c *gin.Context) {
        var order struct {
            Symbol   string  `json:"symbol"`
            IsBuy    bool    `json:"isBuy"`
            Quantity int     `json:"quantity"`
            Price    float64 `json:"price"`
            UserID   interface{} `json:"user_id"` // Optional: if provided, use this user (can be string or number)
        }

        if err := c.ShouldBindJSON(&order); err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid order data: %v", err.Error())})
            return
        }

        log.Printf("DEBUG: Received order request: symbol=%s, isBuy=%t, quantity=%d, price=%f, userID=%v",
            order.Symbol, order.IsBuy, order.Quantity, order.Price, order.UserID)

        if order.Symbol == "" || order.Quantity <= 0 || order.Price <= 0 {
            c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid order parameters: symbol=%s, quantity=%d, price=%f", order.Symbol, order.Quantity, order.Price)})
            return
        }

        // Determine the user ID to use
        var userID string
        if order.UserID != nil {
            // Convert interface{} to string
            switch v := order.UserID.(type) {
            case string:
                userID = v
            case float64: // JSON numbers are parsed as float64
                userID = strconv.Itoa(int(v))
            default:
                userID = fmt.Sprintf("%v", v)
            }
        }

        if userID == "" {
            // Try to get user ID from JWT token if available
            userIDInterface, exists := c.Get("userID")
            if exists {
                userID = strconv.Itoa(userIDInterface.(int))
            } else {
                // Fallback to system_bot if no user specified
                userID = "system_bot"
            }
        }

        // Check if order would be rejected due to insufficient balance BEFORE any database operations
        if order.IsBuy {
            // Convert userID to integer for PostgreSQL users
            var userIDInt int
            var err error
            if userIDInt, err = strconv.Atoi(userID); err == nil && userIDInt > 0 {
                user, err := postgresUserService.GetUserByID(userIDInt)
                if err == nil {
                    requiredBalance := float64(order.Quantity) * order.Price
                    if user.Balance < requiredBalance {
                        c.JSON(http.StatusBadRequest, gin.H{
                            "error": fmt.Sprintf("Insufficient balance. Required: $%.2f, Available: $%.2f", requiredBalance, user.Balance),
                        })
                        return
                    }
                }
            }
        } else {
            // For SELL orders, check if user has enough stock to sell
            var userIDInt int
            var err error
            if userIDInt, err = strconv.Atoi(userID); err == nil && userIDInt > 0 {
                // Get the user's current stock ownership for this symbol
                ownedQuantity, err := postgresUserService.GetStockQuantity(userIDInt, order.Symbol)
                if err != nil {
                    log.Printf("Warning: Failed to get stock ownership for user %d: %v", userIDInt, err)
                    // If we can't get stock ownership, assume they don't own any
                    ownedQuantity = 0
                }

                // Check if user is trying to sell more than they own
                if order.Quantity > ownedQuantity {
                    c.JSON(http.StatusBadRequest, gin.H{
                        "error": fmt.Sprintf("Insufficient stock ownership. Trying to sell %d shares of %s, but only own %d shares", order.Quantity, order.Symbol, ownedQuantity),
                    })
                    return
                }
            }
            // For system_bot, we don't check stock ownership (infinite stocks)
        }

        // Create order record in database for all users (including system_bot)
        var dbOrderID int
        orderType := "BUY"
        if !order.IsBuy {
            orderType = "SELL"
        }

        // Try to create order record in database for numeric user IDs (PostgreSQL users)
        if numericUserID, err := strconv.Atoi(userID); err == nil {
            // For buy orders, deduct from balance immediately (they pay when placing the order)
            // For sell orders, don't add to balance yet (they get paid when the order is matched)
            if order.IsBuy {
                // Calculate the amount to deduct from balance for buy orders
                amount := float64(order.Quantity) * order.Price * -1

                // Create order record in database
                dbOrder, err := orderService.CreateOrder(numericUserID, order.Symbol, orderType, order.Quantity, order.Price, "active")
                if err != nil {
                    log.Printf("Warning: Failed to create order record for user %d: %v", numericUserID, err)
                } else {
                    dbOrderID = dbOrder.ID
                    log.Printf("INFO: Created order %d in database for user %d", dbOrderID, numericUserID)
                }

                // Update the user's balance in PostgreSQL for buy orders only
                err = postgresUserService.UpdateUserBalance(numericUserID, amount)
                if err != nil {
                    log.Printf("Warning: Failed to update balance for user %d: %v", numericUserID, err)
                }
            } else {
                // For sell orders, create the order record but don't update balance yet
                // The balance will be updated when the order is matched in the exchange
                dbOrder, err := orderService.CreateOrder(numericUserID, order.Symbol, orderType, order.Quantity, order.Price, "active")
                if err != nil {
                    log.Printf("Warning: Failed to create order record for user %d: %v", numericUserID, err)
                } else {
                    dbOrderID = dbOrder.ID
                    log.Printf("INFO: Created order %d in database for user %d (sell order - balance update deferred)", dbOrderID, numericUserID)
                }
            }
        } else {
            // For non-numeric user IDs (like system_bot), create a special order record
            // We'll use user_id = 0 for system_bot orders to track them in the database
            dbOrder, err := orderService.CreateOrder(0, order.Symbol, orderType, order.Quantity, order.Price, "active")
            if err != nil {
                log.Printf("Warning: Failed to create order record for system_bot: %v", err)
            } else {
                dbOrderID = dbOrder.ID
                log.Printf("INFO: Created order %d in database for system_bot", dbOrderID)
            }
        }

        // Create and add order to the exchange
        engineOrder := &engine.Order{
            Id:       "order_" + time.Now().Format("20060102150405"),
            Symbol:   order.Symbol,
            IsBuy:    order.IsBuy,
            Quantity: order.Quantity,
            Price:    uint64(order.Price),
            TimeStamp: time.Now(),
            UserID:   userID,
            DBOrderID: dbOrderID, // Store the database order ID
        }

        log.Printf("DEBUG: Passing order %s to exchange engine, dbOrderID=%d", engineOrder.Id, engineOrder.DBOrderID)
        ex.RouteOrder(engineOrder)
        log.Printf("DEBUG: Exchange engine processing completed for order %s", engineOrder.Id)

        c.JSON(http.StatusOK, gin.H{
            "message": "Order placed successfully",
            "order": gin.H{
                "symbol": order.Symbol,
                "type": func() string {
                    if order.IsBuy {
                        return "BUY"
                    }
                    return "SELL"
                }(),
                "quantity": order.Quantity,
                "price": order.Price,
                "user_id": userID,
            },
        })
    }
}





func CompleteOrderHandler(orderService *services.OrderService) gin.HandlerFunc {
    return func(c *gin.Context) {
        orderID := c.Param("id")
        if orderID == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Missing order ID parameter"})
            return
        }

        // Convert orderID to integer
        orderIDInt, err := strconv.Atoi(orderID)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
            return
        }

        err = orderService.CompleteOrder(orderIDInt)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete order"})
            return
        }

        c.JSON(http.StatusOK, gin.H{
            "message": "Order marked as completed successfully",
        })
    }
}

func CancelOrderHandler(orderService *services.OrderService) gin.HandlerFunc {
    return func(c *gin.Context) {
          orderID := c.Param("id")
        if orderID == "" {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Missing order ID parameter"})
            return
        }

        // Convert orderID to integer
        orderIDInt, err := strconv.Atoi(orderID)
        if err != nil {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
            return
        }

        err = orderService.CancelOrder(orderIDInt)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order"})
            return
        }

        c.JSON(http.StatusOK, gin.H{
            "message": "Order marked as cancelled successfully",
        })
    }
}

func ProfileHandler(postgresUserService *services.PostgresUserService) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Get user ID from JWT token
        userIDInterface, exists := c.Get("userID")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
            return
        }

        userID := userIDInterface.(int)

        // Get user from database
        user, err := postgresUserService.GetUserByID(userID)
        if err != nil {
            c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
            return
        }

        c.JSON(http.StatusOK, gin.H{
            "user": gin.H{
                "id": user.ID,
                "username": user.Username,
                "email": user.Email,
                "balance": user.Balance,
                "created_at": user.CreatedAt,
                "profit": user.Profit,
                "loss": user.Loss,
            },
        })
    }
}

func GetCurrentStockPricesHandler(ex *engine.Exchange) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Get all symbols from the exchange
        symbols := ex.GetTickers()

        // Build current prices map
        currentPrices := make(map[string]float64)
        for _, symbol := range symbols {
            // Get the current best bid price (what you can sell at)
            ob := ex.GetOrderBook(symbol)
            if ob != nil && len(ob.Bids) > 0 {
                // Use the best bid price (highest buy order)
                for price := range ob.Bids {
                    currentPrices[symbol] = float64(price)
                    break
                }
            } else {
                // Fallback to a reasonable default if no orders
                currentPrices[symbol] = 100.00
            }
        }

        c.JSON(http.StatusOK, gin.H{
            "current_prices": currentPrices,
        })
    }
}

func GetStockOwnershipHandler(postgresUserService *services.PostgresUserService) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Get user ID from JWT token
        userIDInterface, exists := c.Get("userID")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
            return
        }

        userID := userIDInterface.(int)

        // Get all symbols that the user has traded
        symbols, err := postgresUserService.GetAllTradedSymbols(userID)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch stock ownership"})
            return
        }

        // Build stock ownership map
        stockOwnership := make(map[string]int)
        for _, symbol := range symbols {
            // Calculate ownership for this symbol
            quantity, err := postgresUserService.GetStockQuantity(userID, symbol)
            if err != nil {
                continue
            }

            if quantity > 0 {
                stockOwnership[symbol] = quantity
            }
        }

        c.JSON(http.StatusOK, gin.H{
            "user_id": userID,
            "stock_ownership": stockOwnership,
        })
    }
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func StartAPI(ex *engine.Exchange, pls *services.ProfitLossService, postgresUserService *services.PostgresUserService) {
	router := gin.Default()
	router.SetTrustedProxies(nil)

	router.Use(CORSMiddleware())

	// Public routes
	router.GET("/testing", TestingHttp)
	router.GET("/tickers", GetExchangeTickets(ex))
	router.GET("/book", GetOrderBookHandler(ex))
	router.GET("/profit-loss", GetProfitLossHandler(pls, postgresUserService))
	router.GET("/calculate-profit-loss", CalculateProfitLossHandler(pls))
	router.GET("/current-prices", GetCurrentStockPricesHandler(ex))
	

	// Order management routes (all require authentication)
	orderService := services.NewOrderService(database.DB)
	router.GET("/orders/active", JWTAuthMiddleware(), GetActiveOrdersHandler(orderService))
	router.GET("/orders/history", JWTAuthMiddleware(), GetOrderHistoryHandler(orderService))
	router.POST("/order", JWTAuthMiddleware(), PlaceOrderHandler(ex, postgresUserService, orderService))
	router.POST("/orders/:id/complete", JWTAuthMiddleware(), CompleteOrderHandler(orderService))
	router.POST("/orders/:id/cancel", JWTAuthMiddleware(), CancelOrderHandler(orderService))

	// Authentication routes
	authGroup := router.Group("/auth")
	{
		authGroup.POST("/register", RegisterHandler(postgresUserService))
		authGroup.POST("/login", LoginHandler(postgresUserService))
		authGroup.GET("/me", JWTAuthMiddleware(), MeHandler(postgresUserService))
		authGroup.GET("/profile", JWTAuthMiddleware(), ProfileHandler(postgresUserService))
		authGroup.GET("/stock-ownership", JWTAuthMiddleware(), GetStockOwnershipHandler(postgresUserService))
	}


    /*
	// Protected routes (example)
	protectedGroup := router.Group("/protected")
	protectedGroup.Use(JWTAuthMiddleware())
	{
		// Add protected routes here
	}
    */

	router.Run("localhost:8080")
}
