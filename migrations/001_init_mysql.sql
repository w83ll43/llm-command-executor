CREATE TABLE servers (
  id VARCHAR(128) PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  address VARCHAR(255) NOT NULL,
  ssh_user VARCHAR(128) NOT NULL,
  private_key_ref VARCHAR(512) NOT NULL,
  group_name VARCHAR(128) NOT NULL,
  disabled BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE command_specs (
  command_key VARCHAR(128) PRIMARY KEY,
  description TEXT NOT NULL,
  executable VARCHAR(512) NOT NULL,
  args_json JSON NOT NULL,
  validators_json JSON NOT NULL,
  timeout_seconds INT NOT NULL,
  max_output_bytes INT NOT NULL,
  require_async_mode BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

CREATE TABLE api_tokens (
  id VARCHAR(128) PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  token_hash CHAR(64) NOT NULL,
  role_name VARCHAR(128) NOT NULL,
  disabled BOOLEAN NOT NULL DEFAULT FALSE,
  expires_at TIMESTAMP NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE KEY idx_api_tokens_hash (token_hash)
);

CREATE TABLE policies (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  role_name VARCHAR(128) NOT NULL,
  server_id VARCHAR(128) NULL,
  server_group VARCHAR(128) NULL,
  command_key VARCHAR(128) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_policies_role (role_name),
  KEY idx_policies_server_id (server_id),
  KEY idx_policies_server_group (server_group),
  KEY idx_policies_command_key (command_key)
);

CREATE TABLE executions (
  id VARCHAR(128) PRIMARY KEY,
  server_id VARCHAR(128) NOT NULL,
  command_key VARCHAR(128) NOT NULL,
  args_json JSON NOT NULL,
  mode VARCHAR(16) NOT NULL,
  status VARCHAR(32) NOT NULL,
  exit_code INT NOT NULL DEFAULT 0,
  stdout MEDIUMTEXT NULL,
  stderr MEDIUMTEXT NULL,
  error_message TEXT NULL,
  rendered TEXT NULL,
  caller_token_id VARCHAR(128) NOT NULL,
  caller_name VARCHAR(255) NOT NULL,
  started_at TIMESTAMP NOT NULL,
  finished_at TIMESTAMP NULL,
  KEY idx_executions_server (server_id),
  KEY idx_executions_command (command_key),
  KEY idx_executions_status (status),
  KEY idx_executions_started (started_at)
);

CREATE TABLE audit_events (
  id VARCHAR(128) PRIMARY KEY,
  kind VARCHAR(64) NOT NULL,
  execution_id VARCHAR(128) NULL,
  caller VARCHAR(255) NULL,
  message TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_audit_execution (execution_id),
  KEY idx_audit_kind (kind),
  KEY idx_audit_created (created_at)
);

CREATE TABLE hook_runs (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  hook_name VARCHAR(128) NOT NULL,
  phase VARCHAR(64) NOT NULL,
  execution_id VARCHAR(128) NULL,
  status VARCHAR(32) NOT NULL,
  error_message TEXT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  KEY idx_hook_runs_execution (execution_id),
  KEY idx_hook_runs_phase (phase)
);
