package executor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"llm-command-executor/internal/domain"
)

type SSHConfig struct {
	KnownHostsPath           string
	InsecureSkipHostKeyCheck bool
}

type SSHExecutor struct {
	config SSHConfig
}

func NewSSHExecutor(config SSHConfig) (*SSHExecutor, error) {
	if !config.InsecureSkipHostKeyCheck && config.KnownHostsPath == "" {
		return nil, errors.New("known_hosts_path is required unless insecure_skip_host_key_check is true")
	}
	return &SSHExecutor{config: config}, nil
}

func (e *SSHExecutor) Execute(ctx context.Context, server domain.Server, commandLine string, maxOutputBytes int) (Result, error) {
	key, err := os.ReadFile(server.PrivateKeyPath)
	if err != nil {
		return Result{}, fmt.Errorf("read private key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return Result{}, fmt.Errorf("parse private key: %w", err)
	}
	hostKeyCallback, err := e.hostKeyCallback()
	if err != nil {
		return Result{}, err
	}

	clientConfig := &ssh.ClientConfig{
		User:            server.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", server.Address)
	if err != nil {
		return Result{}, fmt.Errorf("dial ssh: %w", err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, server.Address, clientConfig)
	if err != nil {
		conn.Close()
		return Result{}, fmt.Errorf("handshake ssh: %w", err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return Result{}, fmt.Errorf("new ssh session: %w", err)
	}
	defer session.Close()

	stdout := NewLimitBuffer(maxOutputBytes)
	stderr := NewLimitBuffer(maxOutputBytes)
	session.Stdout = stdout
	session.Stderr = stderr

	done := make(chan error, 1)
	go func() {
		done <- session.Run(commandLine)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		_ = client.Close()
		return Result{Stdout: stdout.String(), Stderr: stderr.String()}, ctx.Err()
	case err := <-done:
		result := Result{ExitCode: exitCode(err), Stdout: stdout.String(), Stderr: stderr.String()}
		if stdout.Exceeded() {
			return result, OutputLimitError("stdout", maxOutputBytes)
		}
		if stderr.Exceeded() {
			return result, OutputLimitError("stderr", maxOutputBytes)
		}
		if err != nil {
			return result, err
		}
		return result, nil
	}
}

func (e *SSHExecutor) hostKeyCallback() (ssh.HostKeyCallback, error) {
	if e.config.InsecureSkipHostKeyCheck {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	callback, err := knownhosts.New(e.config.KnownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts: %w", err)
	}
	return callback, nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *ssh.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitStatus()
	}
	return -1
}
