package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	ratelimit "github.com/vnmchuo/ratelimiter"
	ratelimitgin "github.com/vnmchuo/ratelimiter/middleware/gin"
)

func main() {
	// 1. Initialize Redis Client
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// 2. Initialize Rate Limiter (e.g., 5 requests per 10 seconds)
	limiter := ratelimit.NewRedisStore(
		rdb,
		ratelimit.WithLimit(5),
		ratelimit.WithWindow(10*time.Second),
	)

	// 3. Setup Gin Engine
	r := gin.Default()

	// 4. Define how to identify the user (e.g., via IP address)
	keyFunc := func(c *gin.Context) string {
		return c.ClientIP()
	}

	// 5. Apply Middleware to specific routes or globally
	r.Use(ratelimitgin.RateLimiter(limiter, keyFunc))

	// 6. Define Endpoint
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	log.Println("Server running on :8080")
	r.Run(":8080")
}
