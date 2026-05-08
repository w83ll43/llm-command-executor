package service

import (
	"context"
	"testing"

	"llm-command-gateway/internal/auth"
	"llm-command-gateway/internal/domain"
	"llm-command-gateway/internal/executor"
	"llm-command-gateway/internal/hooks"
	"llm-command-gateway/internal/store"
)

type fakeExecutor struct{}

func (fakeExecutor) Execute(_ context.Context, _ domain.Server, commandLine string, _ int) (executor.Result, error) {
	return executor.Result{ExitCode: 0, Stdout: commandLine}, nil
}

func TestRunSyncExecution(t *testing.T) {
	token := "secret"
	svc := newTestService(token)
	execution, err := svc.Run(context.Background(), token, domain.ExecutionRequest{
		ServerID:   "dev",
		CommandKey: "disk_usage",
		Args:       map[string]string{"path": "/"},
		Mode:       domain.ExecutionModeSync,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if execution.Status != domain.ExecutionSucceeded {
		t.Fatalf("unexpected status: %s", execution.Status)
	}
	if execution.Stdout != "/usr/bin/df -h /" {
		t.Fatalf("unexpected stdout: %s", execution.Stdout)
	}
}

func TestRunRejectsUnauthorizedCommand(t *testing.T) {
	token := "secret"
	svc := newTestService(token)
	_, err := svc.Run(context.Background(), token, domain.ExecutionRequest{
		ServerID:   "dev",
		CommandKey: "systemctl_status",
		Args:       map[string]string{"service": "nginx"},
	})
	if err == nil {
		t.Fatal("expected authorization error")
	}
}

func newTestService(token string) *Service {
	tokens := []domain.APIToken{{ID: "token-1", Name: "test", TokenHash: auth.HashToken(token), Role: "operator"}}
	policies := []domain.Policy{{Role: "operator", ServerGroups: []string{"dev"}, CommandKeys: []string{"disk_usage"}}}
	servers := []domain.Server{{ID: "dev", Group: "dev"}}
	commands := []domain.CommandSpec{
		{
			Key:            "disk_usage",
			Executable:     "/usr/bin/df",
			Args:           []string{"-h", "{{path}}"},
			TimeoutSeconds: 1,
			MaxOutputBytes: 1024,
			Validators: map[string]domain.Validator{
				"path": {Type: "enum", Values: []string{"/"}},
			},
		},
		{
			Key:            "systemctl_status",
			Executable:     "/usr/bin/systemctl",
			Args:           []string{"status", "{{service}}"},
			TimeoutSeconds: 1,
			MaxOutputBytes: 1024,
			Validators: map[string]domain.Validator{
				"service": {Type: "enum", Values: []string{"nginx"}},
			},
		},
	}
	mem := store.NewMemoryStore(servers, commands, tokens, policies)
	return New(mem, auth.NewAuthenticator(tokens), auth.NewAuthorizer(policies), fakeExecutor{}, hooks.NewChain())
}
