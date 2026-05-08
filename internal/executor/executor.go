package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	"llm-command-executor/internal/domain"
)

var ErrOutputLimitExceeded = errors.New("output limit exceeded")

type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type Executor interface {
	Execute(ctx context.Context, server domain.Server, commandLine string, maxOutputBytes int) (Result, error)
}

type LimitBuffer struct {
	mu       sync.Mutex
	buf      bytes.Buffer
	limit    int
	exceeded bool
}

func NewLimitBuffer(limit int) *LimitBuffer {
	return &LimitBuffer{limit: limit}
}

func (b *LimitBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.limit <= 0 {
		return len(p), nil
	}
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.exceeded = true
		return len(p), ErrOutputLimitExceeded
	}
	if len(p) > remaining {
		b.buf.Write(p[:remaining])
		b.exceeded = true
		return len(p), ErrOutputLimitExceeded
	}
	return b.buf.Write(p)
}

func (b *LimitBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *LimitBuffer) Exceeded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.exceeded
}

func OutputLimitError(stream string, limit int) error {
	return fmt.Errorf("%w: %s exceeded %d bytes", ErrOutputLimitExceeded, stream, limit)
}
