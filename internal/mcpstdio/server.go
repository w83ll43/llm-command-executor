package mcpstdio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"llm-command-gateway/internal/domain"
	"llm-command-gateway/internal/service"
)

type Server struct {
	service *service.Service
	token   string
	logger  *slog.Logger
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewServer(service *service.Service, token string, logger *slog.Logger) *Server {
	return &Server{service: service, token: token, logger: logger}
}

func (s *Server) Serve(in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		msg, err := readMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if msg.ID == nil {
			continue
		}
		resp := s.handle(context.Background(), msg)
		if err := writeMessage(out, resp); err != nil {
			return err
		}
	}
}

func (s *Server) handle(ctx context.Context, msg rpcMessage) rpcMessage {
	resp := rpcMessage{JSONRPC: "2.0", ID: msg.ID}
	switch msg.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]string{
				"name":    "llm-command-gateway",
				"version": "0.1.0",
			},
		}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": s.tools()}
	case "tools/call":
		result, err := s.callTool(ctx, msg.Params)
		if err != nil {
			resp.Error = &rpcError{Code: -32000, Message: err.Error()}
		} else {
			resp.Result = result
		}
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found"}
	}
	return resp
}

func (s *Server) tools() []map[string]any {
	return []map[string]any{
		{
			"name":        "list_allowed_commands",
			"description": "List command templates available to the configured MCP token.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "list_allowed_servers",
			"description": "List servers available to the configured MCP token.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "run_command",
			"description": "Run a whitelisted command template on an authorized server.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"server_id":   map[string]any{"type": "string"},
					"command_key": map[string]any{"type": "string"},
					"args":        map[string]any{"type": "object"},
					"mode":        map[string]any{"type": "string", "enum": []string{"sync", "async"}},
				},
				"required": []string{"server_id", "command_key", "args"},
			},
		},
		{
			"name":        "get_execution",
			"description": "Get an execution by id.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "string"},
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "cancel_execution",
			"description": "Cancel a running async execution.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "string"},
				},
				"required": []string{"id"},
			},
		},
	}
}

func (s *Server) callTool(ctx context.Context, raw json.RawMessage) (map[string]any, error) {
	var req struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, err
	}
	var result any
	var err error
	switch req.Name {
	case "list_allowed_commands":
		result, err = s.service.ListAllowedCommands(ctx, s.token)
	case "list_allowed_servers":
		result, err = s.service.ListAllowedServers(ctx, s.token)
	case "run_command":
		var execReq domain.ExecutionRequest
		if err := json.Unmarshal(req.Arguments, &execReq); err != nil {
			return nil, err
		}
		result, err = s.service.Run(ctx, s.token, execReq)
	case "get_execution":
		var args struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			return nil, err
		}
		result, err = s.service.GetExecution(ctx, s.token, args.ID)
	case "cancel_execution":
		var args struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			return nil, err
		}
		result, err = s.service.Cancel(ctx, s.token, args.ID)
	default:
		return nil, fmt.Errorf("unknown tool %q", req.Name)
	}
	if err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []map[string]string{
			{"type": "text", "text": string(content)},
		},
	}, nil
}

func readMessage(reader *bufio.Reader) (rpcMessage, error) {
	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return rpcMessage{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return rpcMessage{}, fmt.Errorf("invalid header %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			n, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return rpcMessage{}, fmt.Errorf("invalid content length: %w", err)
			}
			contentLength = n
		}
	}
	if contentLength < 0 {
		return rpcMessage{}, errors.New("missing Content-Length header")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, body); err != nil {
		return rpcMessage{}, err
	}
	var msg rpcMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return rpcMessage{}, err
	}
	return msg, nil
}

func writeMessage(out io.Writer, msg rpcMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Content-Length: %d\r\n\r\n", len(body))
	buf.Write(body)
	_, err = out.Write(buf.Bytes())
	return err
}
