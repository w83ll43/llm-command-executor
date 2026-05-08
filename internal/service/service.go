package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"llm-command-executor/internal/auth"
	"llm-command-executor/internal/domain"
	"llm-command-executor/internal/executor"
	"llm-command-executor/internal/hooks"
	"llm-command-executor/internal/policy"
)

type Store interface {
	ListServers(context.Context) ([]domain.Server, error)
	GetServer(context.Context, string) (domain.Server, error)
	ListCommands(context.Context) ([]domain.CommandSpec, error)
	GetCommand(context.Context, string) (domain.CommandSpec, error)
	CreateExecution(context.Context, domain.Execution) (domain.Execution, error)
	UpdateExecution(context.Context, domain.Execution) error
	GetExecution(context.Context, string) (domain.Execution, error)
	AppendAudit(context.Context, domain.AuditEvent) error
}

type Service struct {
	store      Store
	authn      *auth.Authenticator
	authz      *auth.Authorizer
	executor   executor.Executor
	hooks      *hooks.Chain
	cancelMu   sync.Mutex
	cancellers map[string]context.CancelFunc
}

func New(store Store, authn *auth.Authenticator, authz *auth.Authorizer, executor executor.Executor, chain *hooks.Chain) *Service {
	return &Service{
		store:      store,
		authn:      authn,
		authz:      authz,
		executor:   executor,
		hooks:      chain,
		cancellers: map[string]context.CancelFunc{},
	}
}

func (s *Service) ListAllowedCommands(ctx context.Context, bearer string) ([]domain.CommandSpec, error) {
	principal, err := s.authn.Authenticate(ctx, bearer)
	if err != nil {
		return nil, err
	}
	commands, err := s.store.ListCommands(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.CommandSpec, 0, len(commands))
	for _, command := range commands {
		if s.authz.CanSeeCommand(principal, command) {
			out = append(out, command)
		}
	}
	return out, nil
}

func (s *Service) ListAllowedServers(ctx context.Context, bearer string) ([]domain.Server, error) {
	principal, err := s.authn.Authenticate(ctx, bearer)
	if err != nil {
		return nil, err
	}
	servers, err := s.store.ListServers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Server, 0, len(servers))
	for _, server := range servers {
		if !server.Disabled && s.authz.CanSeeServer(principal, server) {
			out = append(out, sanitizeServer(server))
		}
	}
	return out, nil
}

func (s *Service) Run(ctx context.Context, bearer string, req domain.ExecutionRequest) (domain.Execution, error) {
	principal, err := s.authn.Authenticate(ctx, bearer)
	if err != nil {
		return domain.Execution{}, err
	}
	if req.Mode == "" {
		req.Mode = domain.ExecutionModeSync
	}
	if req.Mode != domain.ExecutionModeSync && req.Mode != domain.ExecutionModeAsync {
		return domain.Execution{}, fmt.Errorf("unsupported execution mode %q", req.Mode)
	}

	server, command, rendered, err := s.prepare(ctx, principal, req)
	execution := domain.Execution{
		ServerID:   req.ServerID,
		CommandKey: req.CommandKey,
		Args:       req.Args,
		Mode:       req.Mode,
		Status:     domain.ExecutionRunning,
		Rendered:   rendered.Line,
		Caller:     principal,
		StartedAt:  time.Now(),
	}
	if err != nil {
		execution.Status = domain.ExecutionRejected
		execution.ErrorMessage = err.Error()
		created, createErr := s.store.CreateExecution(ctx, execution)
		if createErr == nil {
			_ = s.audit(ctx, "rejected", created, principal, err.Error())
		}
		if createErr != nil {
			return domain.Execution{}, createErr
		}
		return created, err
	}

	execution, err = s.store.CreateExecution(ctx, execution)
	if err != nil {
		return domain.Execution{}, err
	}
	if err := s.audit(ctx, "accepted", execution, principal, "execution accepted"); err != nil {
		return domain.Execution{}, err
	}

	if req.Mode == domain.ExecutionModeAsync || command.RequireAsyncMode {
		execution.Status = domain.ExecutionQueued
		if err := s.store.UpdateExecution(ctx, execution); err != nil {
			return domain.Execution{}, err
		}
		go s.execute(context.Background(), execution, principal, server, command, rendered)
		return execution, nil
	}

	return s.execute(ctx, execution, principal, server, command, rendered)
}

func (s *Service) GetExecution(ctx context.Context, bearer string, id string) (domain.Execution, error) {
	principal, err := s.authn.Authenticate(ctx, bearer)
	if err != nil {
		return domain.Execution{}, err
	}
	execution, err := s.store.GetExecution(ctx, id)
	if err != nil {
		return domain.Execution{}, err
	}
	if execution.Caller.TokenID != principal.TokenID {
		return domain.Execution{}, auth.ErrForbidden
	}
	return execution, nil
}

func (s *Service) Cancel(ctx context.Context, bearer string, id string) (domain.Execution, error) {
	principal, err := s.authn.Authenticate(ctx, bearer)
	if err != nil {
		return domain.Execution{}, err
	}
	execution, err := s.store.GetExecution(ctx, id)
	if err != nil {
		return domain.Execution{}, err
	}
	if execution.Caller.TokenID != principal.TokenID {
		return domain.Execution{}, auth.ErrForbidden
	}
	s.cancelMu.Lock()
	cancel, ok := s.cancellers[id]
	s.cancelMu.Unlock()
	if ok {
		cancel()
	}
	if execution.Status == domain.ExecutionQueued || execution.Status == domain.ExecutionRunning {
		now := time.Now()
		execution.Status = domain.ExecutionCanceled
		execution.FinishedAt = &now
		execution.ErrorMessage = "canceled by caller"
		if err := s.store.UpdateExecution(ctx, execution); err != nil {
			return domain.Execution{}, err
		}
		_ = s.audit(ctx, "canceled", execution, execution.Caller, "execution canceled")
	}
	return execution, nil
}

func (s *Service) prepare(ctx context.Context, principal domain.Principal, req domain.ExecutionRequest) (domain.Server, domain.CommandSpec, policy.RenderedCommand, error) {
	server, err := s.store.GetServer(ctx, req.ServerID)
	if err != nil {
		return domain.Server{}, domain.CommandSpec{}, policy.RenderedCommand{}, err
	}
	if server.Disabled {
		return domain.Server{}, domain.CommandSpec{}, policy.RenderedCommand{}, fmt.Errorf("server %q is disabled", server.ID)
	}
	command, err := s.store.GetCommand(ctx, req.CommandKey)
	if err != nil {
		return domain.Server{}, domain.CommandSpec{}, policy.RenderedCommand{}, err
	}
	if err := s.authz.CanExecute(principal, server, command); err != nil {
		return domain.Server{}, domain.CommandSpec{}, policy.RenderedCommand{}, err
	}
	if err := s.hooks.Run(ctx, hooks.Event{Phase: hooks.BeforeValidate, Principal: principal, Server: server, Command: command}); err != nil {
		return domain.Server{}, domain.CommandSpec{}, policy.RenderedCommand{}, err
	}
	rendered, err := policy.Render(command, req.Args)
	if err != nil {
		return domain.Server{}, domain.CommandSpec{}, policy.RenderedCommand{}, err
	}
	return server, command, rendered, nil
}

func (s *Service) execute(parent context.Context, execution domain.Execution, principal domain.Principal, server domain.Server, command domain.CommandSpec, rendered policy.RenderedCommand) (domain.Execution, error) {
	timeout := time.Duration(command.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	s.cancelMu.Lock()
	s.cancellers[execution.ID] = cancel
	s.cancelMu.Unlock()
	defer func() {
		s.cancelMu.Lock()
		delete(s.cancellers, execution.ID)
		s.cancelMu.Unlock()
	}()

	execution.Status = domain.ExecutionRunning
	if err := s.store.UpdateExecution(ctx, execution); err != nil {
		return execution, err
	}
	event := hooks.Event{Phase: hooks.BeforeExecute, Principal: principal, Server: server, Command: command, Execution: execution}
	if err := s.hooks.Run(ctx, event); err != nil {
		return s.fail(ctx, execution, principal, err)
	}

	result, err := s.executor.Execute(ctx, server, rendered.Line, command.MaxOutputBytes)
	now := time.Now()
	execution.ExitCode = result.ExitCode
	execution.Stdout = result.Stdout
	execution.Stderr = result.Stderr
	execution.FinishedAt = &now
	if err == nil {
		execution.Status = domain.ExecutionSucceeded
	} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		execution.Status = domain.ExecutionTimedOut
		execution.ErrorMessage = "execution timed out"
	} else if errors.Is(ctx.Err(), context.Canceled) {
		execution.Status = domain.ExecutionCanceled
		execution.ErrorMessage = "execution canceled"
	} else {
		execution.Status = domain.ExecutionFailed
		execution.ErrorMessage = err.Error()
	}
	if updateErr := s.store.UpdateExecution(context.Background(), execution); updateErr != nil {
		return execution, updateErr
	}
	phase := hooks.AfterExecute
	if execution.Status != domain.ExecutionSucceeded {
		phase = hooks.OnError
	}
	_ = s.hooks.Run(context.Background(), hooks.Event{Phase: phase, Principal: principal, Server: server, Command: command, Execution: execution, Error: err})
	_ = s.audit(context.Background(), string(execution.Status), execution, principal, execution.ErrorMessage)
	if err != nil {
		return execution, err
	}
	return execution, nil
}

func (s *Service) fail(ctx context.Context, execution domain.Execution, principal domain.Principal, err error) (domain.Execution, error) {
	now := time.Now()
	execution.Status = domain.ExecutionFailed
	execution.ErrorMessage = err.Error()
	execution.FinishedAt = &now
	if updateErr := s.store.UpdateExecution(ctx, execution); updateErr != nil {
		return execution, updateErr
	}
	_ = s.audit(ctx, "failed", execution, principal, err.Error())
	return execution, err
}

func (s *Service) audit(ctx context.Context, kind string, execution domain.Execution, principal domain.Principal, message string) error {
	return s.store.AppendAudit(ctx, domain.AuditEvent{
		Kind:        kind,
		ExecutionID: execution.ID,
		Caller:      principal.Name,
		Message:     message,
		CreatedAt:   time.Now(),
	})
}

func sanitizeServer(server domain.Server) domain.Server {
	server.PrivateKeyPath = ""
	return server
}
