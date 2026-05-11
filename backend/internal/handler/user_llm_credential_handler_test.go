package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devteam/backend/internal/handler/dto"
	"github.com/devteam/backend/pkg/apierror"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubLlmCredSvc struct {
	get   func(ctx context.Context, uid uuid.UUID) (*dto.LlmCredentialsResponse, error)
	patch func(ctx context.Context, uid uuid.UUID, req *dto.PatchLlmCredentialsRequest, ip, ua string) (*dto.LlmCredentialsResponse, error)
}

func (s *stubLlmCredSvc) GetMasked(ctx context.Context, uid uuid.UUID) (*dto.LlmCredentialsResponse, error) {
	if s.get != nil {
		return s.get(ctx, uid)
	}
	return &dto.LlmCredentialsResponse{}, nil
}

func (s *stubLlmCredSvc) Patch(ctx context.Context, uid uuid.UUID, req *dto.PatchLlmCredentialsRequest, ip, ua string) (*dto.LlmCredentialsResponse, error) {
	if s.patch != nil {
		return s.patch(ctx, uid, req, ip, ua)
	}
	return &dto.LlmCredentialsResponse{}, nil
}

func TestUserLlmCredentialHandler_Patch_BodyTooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUserLlmCredentialHandler(&stubLlmCredSvc{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := bytes.Repeat([]byte("a"), maxLlmCredentialPatchBody+1)
	req := httptest.NewRequest(http.MethodPatch, "/me/llm-credentials", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("userID", uuid.New())

	h.Patch(c)
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	var er apierror.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	assert.Equal(t, apierror.ErrRequestEntityTooLarge, er.Error)
}

func TestUserLlmCredentialHandler_Patch_ResponseBodyNoSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "12345678901234567890SECRETLOG"
	h := NewUserLlmCredentialHandler(&stubLlmCredSvc{
		patch: func(ctx context.Context, uid uuid.UUID, req *dto.PatchLlmCredentialsRequest, ip, ua string) (*dto.LlmCredentialsResponse, error) {
			return &dto.LlmCredentialsResponse{}, nil
		},
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPatch, "/me/llm-credentials", strings.NewReader(`{"openai_api_key":"`+secret+`"}`))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("userID", uuid.New())

	h.Patch(c)
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), secret)
}

func TestUserLlmCredentialHandler_Patch_EmptyJSONObject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUserLlmCredentialHandler(&stubLlmCredSvc{
		patch: func(ctx context.Context, uid uuid.UUID, req *dto.PatchLlmCredentialsRequest, ip, ua string) (*dto.LlmCredentialsResponse, error) {
			assert.NotNil(t, req)
			return &dto.LlmCredentialsResponse{}, nil
		},
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPatch, "/me/llm-credentials", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("userID", uuid.New())
	h.Patch(c)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserLlmCredentialHandler_Patch_EmptyBodyWhitespace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUserLlmCredentialHandler(&stubLlmCredSvc{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPatch, "/me/llm-credentials", strings.NewReader("   \n  "))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("userID", uuid.New())
	h.Patch(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUserLlmCredentialHandler_Patch_UnknownJSONField(t *testing.T) {
	_, err := dto.DecodePatchLlmCredentialsJSON([]byte(`{"openai_api_key":"12345678901234567890x","extra":1}`))
	require.Error(t, err)
}

func TestUserLlmCredentialHandler_Patch_WrongContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUserLlmCredentialHandler(&stubLlmCredSvc{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPatch, "/me/llm-credentials", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "text/plain")
	c.Request = req
	c.Set("userID", uuid.New())
	h.Patch(c)
	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)
	var er apierror.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	assert.Equal(t, apierror.ErrUnsupportedMediaType, er.Error)
}

func TestUserLlmCredentialHandler_Patch_UserIDInJSONRejected(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUserLlmCredentialHandler(&stubLlmCredSvc{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	other := uuid.New()
	body := fmt.Sprintf(`{"user_id":"%s","openai_api_key":"12345678901234567890x"}`, other.String())
	req := httptest.NewRequest(http.MethodPatch, "/me/llm-credentials", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("userID", uuid.New())
	h.Patch(c)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	var er apierror.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &er))
	assert.Equal(t, apierror.ErrBadRequest, er.Error)
}
