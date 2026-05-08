package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"llm-command-gateway/internal/domain"
)

var (
	ErrMissingToken = errors.New("missing bearer token")
	ErrInvalidToken = errors.New("invalid bearer token")
	ErrForbidden    = errors.New("forbidden")
)

type TokenStore interface {
	ListTokens(ctx context.Context) ([]domain.APIToken, error)
	ListPolicies(ctx context.Context) ([]domain.Policy, error)
}

type Authenticator struct {
	tokens []domain.APIToken
}

func NewAuthenticator(tokens []domain.APIToken) *Authenticator {
	return &Authenticator{tokens: tokens}
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (a *Authenticator) Authenticate(_ context.Context, bearer string) (domain.Principal, error) {
	if bearer == "" {
		return domain.Principal{}, ErrMissingToken
	}
	hash := HashToken(bearer)
	now := time.Now()
	for _, token := range a.tokens {
		if token.Disabled {
			continue
		}
		if token.ExpiresAt != nil && now.After(*token.ExpiresAt) {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(hash), []byte(token.TokenHash)) == 1 {
			return domain.Principal{TokenID: token.ID, Name: token.Name, Role: token.Role}, nil
		}
	}
	return domain.Principal{}, ErrInvalidToken
}

type Authorizer struct {
	policies []domain.Policy
}

func NewAuthorizer(policies []domain.Policy) *Authorizer {
	return &Authorizer{policies: policies}
}

func (a *Authorizer) CanExecute(principal domain.Principal, server domain.Server, command domain.CommandSpec) error {
	for _, policy := range a.policies {
		if policy.Role != principal.Role {
			continue
		}
		if matches(policy.ServerIDs, server.ID) || matches(policy.ServerGroups, server.Group) {
			if matches(policy.CommandKeys, command.Key) {
				return nil
			}
		}
	}
	return fmt.Errorf("%w: role %q cannot execute %q on %q", ErrForbidden, principal.Role, command.Key, server.ID)
}

func (a *Authorizer) CanSeeServer(principal domain.Principal, server domain.Server) bool {
	for _, policy := range a.policies {
		if policy.Role != principal.Role {
			continue
		}
		if matches(policy.ServerIDs, server.ID) || matches(policy.ServerGroups, server.Group) {
			return true
		}
	}
	return false
}

func (a *Authorizer) CanSeeCommand(principal domain.Principal, command domain.CommandSpec) bool {
	for _, policy := range a.policies {
		if policy.Role != principal.Role {
			continue
		}
		if matches(policy.CommandKeys, command.Key) {
			return true
		}
	}
	return false
}

func matches(values []string, target string) bool {
	for _, value := range values {
		if value == "*" || value == target {
			return true
		}
	}
	return false
}
