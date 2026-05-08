package auth

import (
	"context"
	"testing"

	"llm-command-gateway/internal/domain"
)

func TestAuthenticateTokenHash(t *testing.T) {
	authn := NewAuthenticator([]domain.APIToken{
		{ID: "t1", Name: "test", TokenHash: HashToken("secret"), Role: "operator"},
	})
	principal, err := authn.Authenticate(context.Background(), "secret")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if principal.Role != "operator" {
		t.Fatalf("unexpected role: %s", principal.Role)
	}
}

func TestAuthorizeRolePolicy(t *testing.T) {
	authz := NewAuthorizer([]domain.Policy{
		{Role: "operator", ServerGroups: []string{"dev"}, CommandKeys: []string{"disk_usage"}},
	})
	err := authz.CanExecute(
		domain.Principal{Role: "operator"},
		domain.Server{ID: "s1", Group: "dev"},
		domain.CommandSpec{Key: "disk_usage"},
	)
	if err != nil {
		t.Fatalf("CanExecute returned error: %v", err)
	}
}
