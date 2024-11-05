package main

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type Message struct {
	Status string `json:"status"`
	Body   string `json:"body"`
}

// Middleware untuk rate limiting per IP
func perClientRateLimiter(r rate.Limit, b int) gin.HandlerFunc {
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}
	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)
	go func() {
		for {
			time.Sleep(time.Minute)
			// Lock the mutex to protect this section from race conditions.
			mu.Lock()
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		// Extract the IP address from the request.
		ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			c.JSON(500, gin.H{"status": "error", "message": "Unable to parse IP address"})
			c.Abort()
			return
		}
		// Lock the mutex to protect this section from race conditions.
		mu.Lock()
		if _, found := clients[ip]; !found {
			clients[ip] = &client{limiter: rate.NewLimiter(r, b)}
		}
		clients[ip].lastSeen = time.Now()
		if !clients[ip].limiter.Allow() {
			mu.Unlock()

			message := Message{
				Status: "Request Failed",
				Body:   "The API is at capacity, try again later.",
			}

			c.JSON(429, message)
			c.Abort()
			return
		}
		mu.Unlock()
		c.Next()
	}
}

func normalEndpointHandler(c *gin.Context) {
	message := Message{
		Status: "Successful",
		Body:   "You have accessed the normal endpoint!",
	}
	c.JSON(200, message)
}

func strictEndpointHandler(c *gin.Context) {
	message := Message{
		Status: "Successful",
		Body:   "You have accessed the strict endpoint!",
	}
	c.JSON(200, message)
}

func main() {
	router := gin.Default()

	// Endpoint dengan rate limit longgar (lebih banyak request yang diizinkan)
	router.GET("/ping_normal", perClientRateLimiter(5, 10), normalEndpointHandler)

	// Endpoint dengan rate limit ketat (lebih sedikit request yang diizinkan)
	router.GET("/ping_strict", perClientRateLimiter(1, 2), strictEndpointHandler)

	err := router.Run(":8080")
	if err != nil {
		log.Println("There was an error listening on port :8080", err)
	}
}
