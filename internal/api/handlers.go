package api 

import (
	"Nexus/internal/engine"
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




func StartAPI(ex *engine.Exchange) {
	router := gin.Default()
	router.SetTrustedProxies(nil)

	
	router.GET("/testing", TestingHttp)

	router.GET("/tickers", GetExchangeTickets(ex))


	router.Run("localhost:8080")
}