package api 

import (
	"Nexus/internal/engine"
	"sort"
	"net/http"
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

func StartAPI(ex *engine.Exchange) {
	router := gin.Default()
	router.SetTrustedProxies(nil)

	router.Use(CORSMiddleware())
	
	router.GET("/testing", TestingHttp)
	router.GET("/tickers", GetExchangeTickets(ex))
	router.GET("/book", GetOrderBookHandler(ex))

	router.Run("localhost:8080")
}