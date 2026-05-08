package hooks

import (
	"context"
	"log/slog"
	"sync/atomic"

	"llm-command-gateway/internal/domain"
)

type Phase string

const (
	BeforeValidate Phase = "before_validate"
	BeforeExecute  Phase = "before_execute"
	AfterExecute   Phase = "after_execute"
	OnError        Phase = "on_error"
	OnAudit        Phase = "on_audit"
)

type Event struct {
	Phase     Phase
	Principal domain.Principal
	Server    domain.Server
	Command   domain.CommandSpec
	Execution domain.Execution
	Error     error
}

type Hook interface {
	Name() string
	Handle(ctx context.Context, event Event) error
}

type Chain struct {
	hooks []Hook
}

func NewChain(hooks ...Hook) *Chain {
	return &Chain{hooks: hooks}
}

func (c *Chain) Run(ctx context.Context, event Event) error {
	for _, hook := range c.hooks {
		if err := hook.Handle(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

type LoggingHook struct {
	logger *slog.Logger
}

func NewLoggingHook(logger *slog.Logger) *LoggingHook {
	return &LoggingHook{logger: logger}
}

func (h *LoggingHook) Name() string {
	return "logging"
}

func (h *LoggingHook) Handle(_ context.Context, event Event) error {
	h.logger.Info("execution hook",
		"phase", event.Phase,
		"execution_id", event.Execution.ID,
		"server_id", event.Execution.ServerID,
		"command_key", event.Execution.CommandKey,
		"caller", event.Principal.Name,
	)
	return nil
}

type StatsHook struct {
	started   atomic.Uint64
	succeeded atomic.Uint64
	failed    atomic.Uint64
}

func NewStatsHook() *StatsHook {
	return &StatsHook{}
}

func (h *StatsHook) Name() string {
	return "stats"
}

func (h *StatsHook) Handle(_ context.Context, event Event) error {
	switch event.Phase {
	case BeforeExecute:
		h.started.Add(1)
	case AfterExecute:
		if event.Execution.Status == domain.ExecutionSucceeded {
			h.succeeded.Add(1)
		} else {
			h.failed.Add(1)
		}
	case OnError:
		h.failed.Add(1)
	}
	return nil
}

func (h *StatsHook) Snapshot() map[string]uint64 {
	return map[string]uint64{
		"started":   h.started.Load(),
		"succeeded": h.succeeded.Load(),
		"failed":    h.failed.Load(),
	}
}
