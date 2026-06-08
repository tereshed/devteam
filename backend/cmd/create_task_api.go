package main

import (
	"fmt"
	"time"

	"github.com/devteam/backend/pkg/jwt"
	"github.com/google/uuid"
)

func main() {
	m := jwt.NewManager("your-secret-key-change-in-production-min-32-chars", time.Hour, time.Hour)
	userID := uuid.MustParse("b1530c63-273c-44f7-8b99-851824f5bc9b")
	token, _ := m.GenerateAccessToken(userID, "user")
	fmt.Print(token)
}
