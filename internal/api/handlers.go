package api 

import (
	"fmt"
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

func PlaceOrderHandler(ex *engine.Exchange, postgresUserService *services.PostgresUserService) gin.HandlerFunc {
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

        // Create and add order to the exchange
        engineOrder := &engine.Order{
            Id:       "order_" + time.Now().Format("20060102150405"),
            Symbol:   order.Symbol,
            IsBuy:    order.IsBuy,
            Quantity: order.Quantity,
            Price:    uint64(order.Price * 100), // Convert to cents
            TimeStamp: time.Now(),
            UserID:   userID,
        }

        ex.RouteOrder(engineOrder)

        // Update PostgreSQL user balance if it's a PostgreSQL user (numeric ID)
        if numericUserID, err := strconv.Atoi(userID); err == nil {
            // Calculate the amount to deduct/add from balance
            amount := float64(order.Quantity) * order.Price
            if order.IsBuy {
                // For buy orders, deduct from balance
                amount = -amount
            } else {
                // For sell orders, add to balance
                amount = amount
            }

            // Update the user's balance in PostgreSQL
            err = postgresUserService.UpdateUserBalance(numericUserID, amount)
            if err != nil {
                log.Printf("Warning: Failed to update balance for user %d: %v", numericUserID, err)
            }
        }

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
	router.POST("/order", PlaceOrderHandler(ex, postgresUserService))

	// Authentication routes
	authGroup := router.Group("/auth")
	{
		authGroup.POST("/register", RegisterHandler(postgresUserService))
		authGroup.POST("/login", LoginHandler(postgresUserService))
		authGroup.GET("/me", JWTAuthMiddleware(), MeHandler(postgresUserService))
		authGroup.GET("/guest", CreateGuestUserHandler(postgresUserService))
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
