package main

import (
	"flag"
	"log/slog"
	"os"

	"llm-command-gateway/internal/app"
	"llm-command-gateway/internal/mcpstdio"
)

func main() {
	configPath := flag.String("config", "config.example.json", "path to configuration JSON")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	runtime, err := app.NewRuntime(*configPath, logger)
	if err != nil {
		logger.Error("failed to initialize runtime", "error", err)
		os.Exit(1)
	}

	token := os.Getenv("LCG_MCP_TOKEN")
	if token == "" {
		logger.Warn("LCG_MCP_TOKEN is empty; MCP tools that require auth will fail")
	}

	server := mcpstdio.NewServer(runtime.Service, token, logger)
	if err := server.Serve(os.Stdin, os.Stdout); err != nil {
		logger.Error("mcp server stopped", "error", err)
		os.Exit(1)
	}
}
