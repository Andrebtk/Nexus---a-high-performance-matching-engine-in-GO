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
	"os"
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

        if userID != "system_bot" {
            userIDInt, err := strconv.Atoi(userID)
            if err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
                return
            }
            user, dbErr := postgresUserService.GetUserByID(userIDInt)
            if dbErr == nil {
                profit = user.Profit
                loss = user.Loss
            } else {
                err = dbErr
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
                c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID is required"})
                return
            }
        }
        
        userIDInt, err := strconv.Atoi(userID)
        if err != nil || userIDInt <= 0 {
            c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid numeric user ID"})
            return
        }

		var dbOrder *services.Order
		var dbOrderID int
		var orderCreationFailed = false


		if order.IsBuy {
			// Atomically check balance, deduct, and create order
			var err error
			dbOrder, err = orderService.PlaceBuyOrderAtomically(userIDInt, order.Symbol, order.Quantity, order.Price)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			dbOrderID = dbOrder.ID
			log.Printf("INFO: Created order %d in database for user %d", dbOrderID, userIDInt)
		} else {
			// Atomically check stock ownership and create order
			var err error
			dbOrder, err = orderService.PlaceSellOrderAtomically(userIDInt, order.Symbol, order.Quantity, order.Price)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			dbOrderID = dbOrder.ID
			log.Printf("INFO: Created order %d in database for user %d", dbOrderID, userIDInt)
		}

		// The code below handles order routing to the engine after atomic creation

        // If order creation failed, don't proceed with the order
        if orderCreationFailed {
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "Failed to create order in database. Please try again.",
            })
            return
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

func CancelOrderHandler(ex *engine.Exchange, orderService *services.OrderService, postgresUserService *services.PostgresUserService) gin.HandlerFunc {
	return func(c *gin.Context) {
		orderIDParam := c.Param("id")
		if orderIDParam == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing order ID parameter"})
			return
		}

		orderIDInt, err := strconv.Atoi(orderIDParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
			return
		}

		userIDInterface, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			return
		}
		authUserID := userIDInterface.(int)

		order, err := orderService.GetOrderByID(orderIDInt)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}

		if order.UserID != authUserID {
			c.JSON(http.StatusForbidden, gin.H{"error": "You do not have permission to cancel this order"})
			return
		}

		if order.Status != "active" {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Cannot cancel order with status '%s'", order.Status)})
			return
		}

		// Remove the resting order from the live matching engine first.
		// If this fails, the order was likely just matched — don't touch the DB.
		liveOrder, err := ex.CancelOrder(order.Symbol, orderIDInt)
		if err != nil {
			log.Printf("WARNING: Failed to remove order %d from live order book: %v", orderIDInt, err)
			c.JSON(http.StatusConflict, gin.H{
				"error": "Order could not be cancelled — it may have just been matched. Please refresh.",
			})
			return
		}

		if err := orderService.CancelOrder(orderIDInt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel order"})
			return
		}

		// BUY orders had funds deducted upfront when placed — refund them.
		// SELL orders never had balance credited or stock deducted upfront
		// (that only happens on fill), so there's nothing to refund there.
		if order.OrderType == "BUY" && liveOrder != nil {
			// Refund based on the REMAINING quantity in the live order book,
			// because partial fills have already been settled and refunded!
			refundAmount := float64(liveOrder.Quantity) * order.Price
			if err := postgresUserService.UpdateUserBalance(authUserID, refundAmount); err != nil {
				log.Printf("WARNING: Failed to refund balance for cancelled order %d: %v", orderIDInt, err)
			} else {
				log.Printf("INFO: Successfully refunded $%.2f to user %d for cancelled order %d", refundAmount, authUserID, orderIDInt)
			}
		}

		log.Printf("INFO: Order %d cancelled by user %d", orderIDInt, authUserID)

		c.JSON(http.StatusOK, gin.H{
			"message":  "Order cancelled successfully",
			"order_id": orderIDInt,
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

func GetStockOwnershipHandler(postgresUserService *services.PostgresUserService, orderService *services.OrderService) gin.HandlerFunc {
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

            // Get reserved quantity in active sell orders
            reserved, err := orderService.GetActiveSellQuantity(userID, symbol)
            if err != nil {
                reserved = 0
            }

            // Show available quantity (owned - reserved)
            available := quantity - reserved
            if available > 0 {
                stockOwnership[symbol] = available
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
	router.POST("/orders/:id/cancel", JWTAuthMiddleware(), CancelOrderHandler(ex, orderService, postgresUserService))

		// Authentication routes
		authGroup := router.Group("/auth")
		{
			authGroup.POST("/register", RegisterHandler(postgresUserService))
			authGroup.POST("/login", LoginHandler(postgresUserService))
			authGroup.GET("/me", JWTAuthMiddleware(), MeHandler(postgresUserService))
			authGroup.GET("/profile", JWTAuthMiddleware(), ProfileHandler(postgresUserService))
			authGroup.GET("/stock-ownership", JWTAuthMiddleware(), GetStockOwnershipHandler(postgresUserService, orderService))
		}


    /*
	// Protected routes (example)
	protectedGroup := router.Group("/protected")
	protectedGroup.Use(JWTAuthMiddleware())
	{
		// Add protected routes here
	}
    */

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	router.Run(":" + port)
}
