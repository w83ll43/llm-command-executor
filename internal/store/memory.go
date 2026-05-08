package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"llm-command-gateway/internal/domain"
)

type MemoryStore struct {
	mu         sync.RWMutex
	servers    map[string]domain.Server
	commands   map[string]domain.CommandSpec
	tokens     []domain.APIToken
	policies   []domain.Policy
	executions map[string]domain.Execution
	audits     []domain.AuditEvent
}

func NewMemoryStore(servers []domain.Server, commands []domain.CommandSpec, tokens []domain.APIToken, policies []domain.Policy) *MemoryStore {
	s := &MemoryStore{
		servers:    map[string]domain.Server{},
		commands:   map[string]domain.CommandSpec{},
		tokens:     tokens,
		policies:   policies,
		executions: map[string]domain.Execution{},
	}
	for _, server := range servers {
		s.servers[server.ID] = server
	}
	for _, command := range commands {
		s.commands[command.Key] = command
	}
	return s
}

func (s *MemoryStore) ListServers(_ context.Context) ([]domain.Server, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.Server, 0, len(s.servers))
	for _, server := range s.servers {
		out = append(out, server)
	}
	return out, nil
}

func (s *MemoryStore) GetServer(_ context.Context, id string) (domain.Server, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	server, ok := s.servers[id]
	if !ok {
		return domain.Server{}, fmt.Errorf("server %q not found", id)
	}
	return server, nil
}

func (s *MemoryStore) ListCommands(_ context.Context) ([]domain.CommandSpec, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]domain.CommandSpec, 0, len(s.commands))
	for _, command := range s.commands {
		out = append(out, command)
	}
	return out, nil
}

func (s *MemoryStore) GetCommand(_ context.Context, key string) (domain.CommandSpec, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	command, ok := s.commands[key]
	if !ok {
		return domain.CommandSpec{}, fmt.Errorf("command %q not found", key)
	}
	return command, nil
}

func (s *MemoryStore) ListTokens(_ context.Context) ([]domain.APIToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.APIToken(nil), s.tokens...), nil
}

func (s *MemoryStore) ListPolicies(_ context.Context) ([]domain.Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.Policy(nil), s.policies...), nil
}

func (s *MemoryStore) CreateExecution(_ context.Context, execution domain.Execution) (domain.Execution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if execution.ID == "" {
		execution.ID = newID("exec")
	}
	if execution.StartedAt.IsZero() {
		execution.StartedAt = time.Now()
	}
	s.executions[execution.ID] = execution
	return execution, nil
}

func (s *MemoryStore) UpdateExecution(_ context.Context, execution domain.Execution) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.executions[execution.ID]; !ok {
		return fmt.Errorf("execution %q not found", execution.ID)
	}
	s.executions[execution.ID] = execution
	return nil
}

func (s *MemoryStore) GetExecution(_ context.Context, id string) (domain.Execution, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	execution, ok := s.executions[id]
	if !ok {
		return domain.Execution{}, fmt.Errorf("execution %q not found", id)
	}
	return execution, nil
}

func (s *MemoryStore) AppendAudit(_ context.Context, event domain.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.ID == "" {
		event.ID = newID("audit")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now()
	}
	s.audits = append(s.audits, event)
	return nil
}

func newID(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}
