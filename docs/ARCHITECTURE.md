# Architecture

## Design Goal

The executor lets an LLM execute operational commands without granting arbitrary shell access. The model can only call named tools that map to configured command templates.

## Runtime Flow

1. REST or MCP receives a request.
2. The bearer token or `LCE_MCP_TOKEN` authenticates the caller.
3. RBAC checks the caller role against the target server and command.
4. The command template validates and renders arguments.
5. Hooks run before validation, before execution, after execution, and on errors.
6. The executor runs the rendered command through SSH.
7. Output, status, duration, and audit events are stored.

## Main Packages

- `internal/domain`: shared domain models.
- `internal/config`: JSON config loading and normalization.
- `internal/auth`: token hashing, authentication, and RBAC.
- `internal/policy`: command template validation and POSIX quoting.
- `internal/executor`: executor interface and SSH implementation.
- `internal/service`: command orchestration and async execution.
- `internal/httpapi`: REST API adapter.
- `internal/mcpstdio`: MCP stdio adapter.
- `internal/hooks`: lifecycle extension points.
- `internal/store`: runtime storage implementation.

## Extension Points

`executor.Executor` is the main boundary for adding Agent-based execution. `hooks.Hook` is the main boundary for callbacks, approval workflows, audit forwarding, and statistics.

## Production Notes

For production, disable `insecure_skip_host_key_check`, use a dedicated SSH user with least privilege, store private keys outside the database, and move from the in-memory store to the MySQL repository planned by the migration schema.
