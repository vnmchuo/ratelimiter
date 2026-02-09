package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	ratelimit "github.com/virgiliusnanamanek02/ratelimiter"
)

func RateLimiter(limiter ratelimit.Limiter, keyFunc func(*gin.Context) string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		key := keyFunc(ctx)
		res, err := limiter.Allow(ctx.Request.Context(), key)

		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "rate limiter error"})
			return
		}

		ctx.Header("X-RateLimit-Limit", fmt.Sprint(res.Limit))
		ctx.Header("X-RateLimit-Remaining", fmt.Sprint(res.Remaining))

		if !res.Allowed {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"message": "too many requests, try again later",
			})
			return
		}

		ctx.Next()

	}
}
