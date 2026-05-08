# LLM Command Gateway

LLM Command Gateway is a controlled remote command execution platform designed for model-driven operations. It exposes the same command service through REST and an MCP stdio server, while enforcing token authentication, RBAC, parameterized command allowlists, output limits, timeouts, hooks, and audit records.

The project intentionally does not expose a raw shell endpoint. Callers select a `command_key` and provide typed arguments that must pass the configured validators.

## Current Scope

- Go implementation targeting Linux servers.
- REST gateway in `cmd/gateway`.
- MCP stdio server in `cmd/mcp`.
- SSH executor using managed private keys.
- In-memory runtime store seeded from JSON config.
- MySQL migration schema in `migrations/` for production persistence work.
- Lifecycle hook chain with logging and statistics examples.

## Quick Start

```powershell
go mod tidy
go test ./...
go run ./cmd/gateway -config config.example.json
```

The example config uses the development token `dev-token-change-me`.

List available commands:

```powershell
Invoke-RestMethod `
  -Headers @{ Authorization = "Bearer dev-token-change-me" } `
  -Uri http://localhost:8080/v1/commands
```

Run a command:

```powershell
Invoke-RestMethod `
  -Method Post `
  -Headers @{ Authorization = "Bearer dev-token-change-me" } `
  -ContentType "application/json" `
  -Body '{"server_id":"dev-linux-01","command_key":"disk_usage","args":{"path":"/"},"mode":"sync"}' `
  -Uri http://localhost:8080/v1/executions
```

Run the MCP stdio server:

```powershell
$env:LCG_MCP_TOKEN = "dev-token-change-me"
go run ./cmd/mcp -config config.example.json
```

Configure an MCP client to launch that command over stdio. The tools exposed are:

- `list_allowed_commands`
- `list_allowed_servers`
- `run_command`
- `get_execution`
- `cancel_execution`

## Command Policy Model

Commands are declared as fixed executables plus argument templates:

```json
{
  "key": "systemctl_status",
  "executable": "/usr/bin/systemctl",
  "args": ["status", "{{service}}"],
  "validators": {
    "service": {
      "type": "enum",
      "values": ["nginx", "mysql", "redis"]
    }
  }
}
```

Supported validators:

- `enum`: value must be in the configured list.
- `regex`: value must match the configured regular expression.
- `string`: value must be non-empty, with optional length limits.

Rendered SSH commands are POSIX quoted after validation.

## Security Defaults

- API tokens are hashed in memory after config load.
- RBAC policies bind roles to server IDs or server groups and command keys.
- Disabled servers and tokens are ignored.
- Unknown command arguments are rejected.
- All accepted, rejected, canceled, failed, timed out, and successful executions are audited.
- SSH host key checking is supported through `known_hosts_path`; the sample config enables insecure checking only for development.

## Roadmap

- Add MySQL-backed repository implementation using the provided schema.
- Add admin REST endpoints for managing servers, commands, policies, and tokens.
- Add Agent executor behind the existing `Executor` interface.
- Add webhook hook implementation for external callbacks.
- Add metrics endpoint and structured audit export.
