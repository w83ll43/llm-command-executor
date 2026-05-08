package policy

import (
	"strings"
	"testing"

	"llm-command-gateway/internal/domain"
)

func TestRenderAllowsValidatedTemplateArgs(t *testing.T) {
	command := domain.CommandSpec{
		Key:        "systemctl_status",
		Executable: "/usr/bin/systemctl",
		Args:       []string{"status", "{{service}}"},
		Validators: map[string]domain.Validator{
			"service": {Type: "enum", Values: []string{"nginx", "redis"}},
		},
	}

	rendered, err := Render(command, map[string]string{"service": "nginx"})
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if rendered.Line != "/usr/bin/systemctl status nginx" {
		t.Fatalf("unexpected command line: %s", rendered.Line)
	}
}

func TestRenderRejectsUnexpectedArg(t *testing.T) {
	command := domain.CommandSpec{
		Key:        "disk_usage",
		Executable: "/usr/bin/df",
		Args:       []string{"-h", "{{path}}"},
		Validators: map[string]domain.Validator{
			"path": {Type: "enum", Values: []string{"/"}},
		},
	}

	_, err := Render(command, map[string]string{"path": "/", "extra": "nope"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unexpected argument") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQuotePOSIXEscapesShellMetacharacters(t *testing.T) {
	got := QuotePOSIX("nginx; rm -rf /")
	if got != "'nginx; rm -rf /'" {
		t.Fatalf("unexpected quote: %s", got)
	}
	got = QuotePOSIX("it's")
	if got != "'it'\\''s'" {
		t.Fatalf("unexpected quote for apostrophe: %s", got)
	}
}
