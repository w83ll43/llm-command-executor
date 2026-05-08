package domain

import "time"

type Server struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Address        string `json:"address"`
	User           string `json:"user"`
	PrivateKeyPath string `json:"private_key_path"`
	Group          string `json:"group"`
	Disabled       bool   `json:"disabled"`
}

type CommandSpec struct {
	Key              string               `json:"key"`
	Description      string               `json:"description"`
	Executable       string               `json:"executable"`
	Args             []string             `json:"args"`
	Validators       map[string]Validator `json:"validators"`
	TimeoutSeconds   int                  `json:"timeout_seconds"`
	MaxOutputBytes   int                  `json:"max_output_bytes"`
	RequireAsyncMode bool                 `json:"require_async_mode"`
}

type Validator struct {
	Type      string   `json:"type"`
	Values    []string `json:"values,omitempty"`
	Pattern   string   `json:"pattern,omitempty"`
	MinLength int      `json:"min_length,omitempty"`
	MaxLength int      `json:"max_length,omitempty"`
}

type APIToken struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Token     string     `json:"token,omitempty"`
	TokenHash string     `json:"token_hash,omitempty"`
	Role      string     `json:"role"`
	Disabled  bool       `json:"disabled"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type Policy struct {
	Role         string   `json:"role"`
	ServerIDs    []string `json:"server_ids,omitempty"`
	ServerGroups []string `json:"server_groups,omitempty"`
	CommandKeys  []string `json:"command_keys,omitempty"`
}

type Principal struct {
	TokenID string `json:"token_id"`
	Name    string `json:"name"`
	Role    string `json:"role"`
}

type ExecutionMode string

const (
	ExecutionModeSync  ExecutionMode = "sync"
	ExecutionModeAsync ExecutionMode = "async"
)

type ExecutionStatus string

const (
	ExecutionQueued    ExecutionStatus = "queued"
	ExecutionRunning   ExecutionStatus = "running"
	ExecutionSucceeded ExecutionStatus = "succeeded"
	ExecutionFailed    ExecutionStatus = "failed"
	ExecutionRejected  ExecutionStatus = "rejected"
	ExecutionCanceled  ExecutionStatus = "canceled"
	ExecutionTimedOut  ExecutionStatus = "timed_out"
)

type ExecutionRequest struct {
	ServerID   string            `json:"server_id"`
	CommandKey string            `json:"command_key"`
	Args       map[string]string `json:"args"`
	Mode       ExecutionMode     `json:"mode"`
}

type Execution struct {
	ID           string            `json:"id"`
	ServerID     string            `json:"server_id"`
	CommandKey   string            `json:"command_key"`
	Args         map[string]string `json:"args"`
	Mode         ExecutionMode     `json:"mode"`
	Status       ExecutionStatus   `json:"status"`
	ExitCode     int               `json:"exit_code"`
	Stdout       string            `json:"stdout,omitempty"`
	Stderr       string            `json:"stderr,omitempty"`
	ErrorMessage string            `json:"error_message,omitempty"`
	Rendered     string            `json:"rendered,omitempty"`
	Caller       Principal         `json:"caller"`
	StartedAt    time.Time         `json:"started_at"`
	FinishedAt   *time.Time        `json:"finished_at,omitempty"`
}

type AuditEvent struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	ExecutionID string    `json:"execution_id,omitempty"`
	Caller      string    `json:"caller,omitempty"`
	Message     string    `json:"message"`
	CreatedAt   time.Time `json:"created_at"`
}
