package service

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

const modelConnectionRetryAttempts = 5

var modelConnectionRetryDelay = 20 * time.Second

func isModelConnectionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	msg := strings.ToLower(err.Error())
	connectionTokens := []string{
		"connection reset",
		"connection refused",
		"broken pipe",
		"server closed idle connection",
		"tls handshake timeout",
		"no such host",
		"i/o timeout",
		"unexpected eof",
		": eof",
	}
	for _, token := range connectionTokens {
		if strings.Contains(msg, token) {
			return true
		}
	}
	return false
}
