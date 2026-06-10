// Package api wires HTTP handlers around the InkOS store + LLM layers.
package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIError is a typed error that maps to a JSON response with status + code.
type APIError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *APIError) Error() string { return e.Message }

// NewAPIError constructs an APIError.
func NewAPIError(status int, code, msg string) *APIError {
	return &APIError{Status: status, Code: code, Message: msg}
}

// Common errors.
var (
	ErrInvalidBookID = NewAPIError(http.StatusBadRequest, "INVALID_BOOK_ID", "Invalid book id")
	ErrBookNotFound  = NewAPIError(http.StatusNotFound, "BOOK_NOT_FOUND", "Book not found")
	ErrNotFound      = NewAPIError(http.StatusNotFound, "NOT_FOUND", "Not found")
	ErrInternal      = NewAPIError(http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
	ErrLLMConfig     = NewAPIError(http.StatusBadRequest, "LLM_CONFIG_ERROR", "LLM not configured")
)

// AbortWithError writes a typed JSON error response and aborts the chain.
func AbortWithError(c *gin.Context, err error) {
	var ae *APIError
	if errors.As(err, &ae) {
		c.AbortWithStatusJSON(ae.Status, gin.H{"error": gin.H{
			"code":    ae.Code,
			"message": ae.Message,
		}})
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": gin.H{
		"code":    "INTERNAL_ERROR",
		"message": err.Error(),
	}})
}

// ErrorHandler is a gin middleware that converts uncaught errors.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		last := c.Errors.Last().Err
		AbortWithError(c, last)
	}
}
