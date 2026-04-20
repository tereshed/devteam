package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/devteam/backend/pkg/jwt"
)

var testJWTManager *jwt.Manager

func init() {
	testJWTManager = jwt.NewManager("test-secret-key-for-testing", time.Hour, 24*time.Hour)
}

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

// Tests for WebSocket subprotocol fallback (bearer.<jwt>)
func TestAuthMiddleware_WS_SubprotocolFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		authHeader     string
		upgrade        string
		subprotocol    string
		expectedStatus int
		expectUserID   bool
	}{
		{
			name:           "Valid JWT via Sec-WebSocket-Protocol with Upgrade: websocket",
			authHeader:     "",
			upgrade:        "websocket",
			subprotocol:    "bearer.test-token",
			expectedStatus: http.StatusOK,
			expectUserID:   true,
		},
		{
			name:           "Sec-WebSocket-Protocol ignored when Upgrade header is missing",
			authHeader:     "",
			upgrade:        "",
			subprotocol:    "bearer.test-token",
			expectedStatus: http.StatusUnauthorized,
			expectUserID:   false,
		},
		{
			name:           "Authorization header takes priority over Sec-WebSocket-Protocol",
			authHeader:     "Bearer valid-token",
			upgrade:        "websocket",
			subprotocol:    "bearer.another-token",
			expectedStatus: http.StatusOK,
			expectUserID:   true, // Uses Authorization, not subprotocol
		},
		{
			name:           "Invalid token in Sec-WebSocket-Protocol returns 401",
			authHeader:     "",
			upgrade:        "websocket",
			subprotocol:    "bearer.invalid-token",
			expectedStatus: http.StatusUnauthorized,
			expectUserID:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/", nil)

			if tt.subprotocol != "" && tt.subprotocol != "bearer.invalid-token" && tt.subprotocol != "bearer.another-token" {
				realToken, _ := testJWTManager.GenerateAccessToken(uuid.New(), "user")
				tt.subprotocol = "bearer." + realToken
			}

			if tt.authHeader != "" && tt.authHeader != "Bearer invalid-token" && tt.authHeader != "Bearer another-token" {
				realToken, _ := testJWTManager.GenerateAccessToken(uuid.New(), "user")
				tt.authHeader = "Bearer " + realToken
			}

			if tt.authHeader != "" {
				c.Request.Header.Set("Authorization", tt.authHeader)
			}
			if tt.upgrade != "" {
				c.Request.Header.Set("Upgrade", tt.upgrade)
			}
			if tt.subprotocol != "" {
				c.Request.Header.Set("Sec-WebSocket-Protocol", tt.subprotocol)
			}

			middleware := AuthMiddleware(testJWTManager, nil)
			middleware(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectUserID {
				_, exists := c.Get("userID")
				assert.True(t, exists, "userID should be set in context")
			}
		})
	}
}

func TestIsWebSocketUpgrade(t *testing.T) {
	tests := []struct {
		upgrade string
		expected bool
	}{
		{"websocket", true},
		{"WebSocket", true},
		{"", false},
		{"http", false},
	}

	for _, tt := range tests {
		t.Run(tt.upgrade, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("Upgrade", tt.upgrade)
			result := isWebSocketUpgrade(r)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractBearerSubprotocol(t *testing.T) {
	tests := []struct {
		name      string
		protocols []string
		expected  string
	}{
		{"single bearer", []string{"bearer.token123"}, "token123"},
		{"bearer with other protocols", []string{"chat", "bearer.token123", "v2"}, "token123"},
		{"no bearer prefix", []string{"chat", "v2"}, ""},
		{"empty bearer", []string{"bearer."}, ""},
		{"bearer not first", []string{"v2", "bearer.token123"}, "token123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			// gorilla/websocket.Subprotocols expects the header to be a single string
			// with comma-separated values, but r.Header.Add adds multiple headers.
			// Let's set it as a single header.
			r.Header.Set("Sec-WebSocket-Protocol", strings.Join(tt.protocols, ", "))
			result := extractBearerSubprotocol(r)
			assert.Equal(t, tt.expected, result)
		})
	}
}
