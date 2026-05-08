package app

import (
	"log/slog"

	"llm-command-gateway/internal/auth"
	"llm-command-gateway/internal/config"
	"llm-command-gateway/internal/executor"
	"llm-command-gateway/internal/hooks"
	"llm-command-gateway/internal/service"
	"llm-command-gateway/internal/store"
)

type Runtime struct {
	Config  *config.Config
	Service *service.Service
}

func NewRuntime(configPath string, logger *slog.Logger) (*Runtime, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	mem := store.NewMemoryStore(cfg.Servers, cfg.Commands, cfg.Tokens, cfg.Policies)
	sshExecutor, err := executor.NewSSHExecutor(executor.SSHConfig{
		KnownHostsPath:           cfg.SSH.KnownHostsPath,
		InsecureSkipHostKeyCheck: cfg.SSH.InsecureSkipHostKeyCheck,
	})
	if err != nil {
		return nil, err
	}
	authn := auth.NewAuthenticator(cfg.Tokens)
	authz := auth.NewAuthorizer(cfg.Policies)
	chain := hooks.NewChain(hooks.NewStatsHook(), hooks.NewLoggingHook(logger))
	return &Runtime{
		Config:  cfg,
		Service: service.New(mem, authn, authz, sshExecutor, chain),
	}, nil
}
