package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAdminOnlyMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		setupContext   func(*gin.Context)
		expectedStatus int
	}{
		{
			name: "Admin user",
			setupContext: func(c *gin.Context) {
				c.Set("userRole", "admin")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "Regular user",
			setupContext: func(c *gin.Context) {
				c.Set("userRole", "user")
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "No role",
			setupContext: func(c *gin.Context) {
				// No role set
			},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			tt.setupContext(c)

			middleware := AdminOnlyMiddleware()
			middleware(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
