package httputil

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

// ParsePagination извлекает и валидирует параметры пагинации из запроса
func ParsePagination(c *gin.Context) (limit, offset int, err error) {
	var query struct {
		Limit  int `form:"limit"`
		Offset int `form:"offset"`
	}
	if err := c.ShouldBindQuery(&query); err != nil {
		return 0, 0, err
	}

	// Валидация пагинации
	if query.Limit < 0 {
		return 0, 0, errors.New("limit must be positive")
	}
	if query.Limit == 0 {
		query.Limit = defaultLimit
	}
	if query.Limit > maxLimit {
		return 0, 0, fmt.Errorf("limit cannot exceed %d", maxLimit)
	}
	if query.Offset < 0 {
		return 0, 0, errors.New("offset must be positive")
	}

	return query.Limit, query.Offset, nil
}
