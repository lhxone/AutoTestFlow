package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
)

func TestIsModelConnectionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "wrapped EOF from model request",
			err:  fmt.Errorf("请求模型服务失败: %w", io.EOF),
			want: true,
		},
		{
			name: "connection reset text",
			err:  errors.New("请求模型服务失败: Post https://example.test: read: connection reset by peer"),
			want: true,
		},
		{
			name: "http status error",
			err:  errors.New("模型服务返回错误 401: invalid api key"),
			want: false,
		},
		{
			name: "context cancellation",
			err:  context.Canceled,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isModelConnectionError(tt.err); got != tt.want {
				t.Fatalf("isModelConnectionError() = %v, want %v", got, tt.want)
			}
		})
	}
}
