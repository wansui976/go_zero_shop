package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zeromicro/go-zero/core/logx"
)

// GetUserIDFromCtx 从context中获取用户ID
func GetUserIDFromCtx(ctx context.Context) (int64, error) {
	uidVal := ctx.Value("uid")
	if uidVal == nil {
		return 0, ErrUnauthorized
	}

	var uid int64
	var err error

	switch v := uidVal.(type) {
	case json.Number:
		uid, err = v.Int64()
	case string:
		uid, err = strconv.ParseInt(v, 10, 64)
	case int64:
		uid = v
	case int:
		uid = int64(v)
	case float64:
		uid = int64(v)
	default:
		logx.Errorf("Invalid uid type: %T", uidVal)
		return 0, ErrUnauthorized
	}

	if err != nil {
		logx.Errorf("Failed to parse uid: %v", err)
		return 0, ErrUnauthorized
	}

	return uid, nil
}

// 自定义错误定义
var (
	ErrUnauthorized = &UnauthorizedError{Message: "Unauthorized"}
)

// UnauthorizedError 自定义未授权错误

type UnauthorizedError struct {
	Message string
}

func (e *UnauthorizedError) Error() string {
	return e.Message
}

func (e *UnauthorizedError) StatusCode() int {
	return http.StatusUnauthorized
}

func (e *UnauthorizedError) Code() int {
	return http.StatusUnauthorized
}
