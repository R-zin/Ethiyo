package main

import (
	"log"
	"main/user"

	"github.com/gin-gonic/gin"
)

func main() {
	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "ok",
		})
	})

	router.GET("/auth/google/login", user.GoogleLogin)
	router.GET("/auth/google/callback", user.GoogleCallback)

	router.GET("/trackroute", func(c *gin.Context) {
		buscode := c.DefaultQuery("buscode", "")
		if buscode == "" {
			log.Fatal("None Returned Query Parameter not passed")
		}
		request(buscode)
		c.JSON(200, gin.H{"message": "ok"})
	})

	router.GET("/trackxhr", func(c *gin.Context) {
		buscode := c.DefaultQuery("buscode", "")
		if buscode == "" {
			log.Fatal("None Returned Query Parameter not passed")
			return
		}
		res, cook, err := getBusRoute(buscode)
		c.JSON(200, gin.H{"message": "ok",
			"route":  res,
			"cookie": cook,
			"error":  err,
		})
	})

	router.GET("/testroute", func(c *gin.Context) {
		buscode := c.Query("buscode")
		bus_url, cookie, err := getBusRoute(buscode)
		if err != nil {
			log.Fatal(err)
		}
		makeReq(bus_url, cookie)
	})
	router.GET("/get_bus_url", func(c *gin.Context) {
		buscode := c.Query("buscode")
		res := get_path_code(buscode)
		c.JSON(200, gin.H{
			"result": res,
		})
	})

	err := router.Run(":8080")
	if err != nil {
		log.Fatal(err)
	}

}
