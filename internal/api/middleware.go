package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/narcooo/inkos/internal/util"
)

// CORS allows all origins (same usage as the original Hono app).
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS,PATCH")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Session-Id")
		c.Header("Access-Control-Expose-Headers", "Content-Type")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// BookIDMiddleware rejects unsafe book ids before reaching handlers.
func BookIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id != "" && !util.IsSafeBookID(id) {
			AbortWithError(c, NewAPIError(http.StatusBadRequest, "INVALID_BOOK_ID",
				"Invalid book id: "+id))
			return
		}
		c.Next()
	}
}
