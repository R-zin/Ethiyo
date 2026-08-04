package main

import (
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "ok",
		})
	})

	router.GET("/trackroute", func(c *gin.Context) {
		buscode := c.DefaultQuery("buscode", "")
		if buscode == "" {
			log.Fatal("None Returned Query Parameter not passed")
		}
		request(buscode)
		c.JSON(200, gin.H{"message": "ok"})
	})
	err := router.Run(":8080")
	if err != nil {
		log.Fatal(err)
	}

}
