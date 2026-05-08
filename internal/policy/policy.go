package policy

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"llm-command-executor/internal/domain"
)

var templatePattern = regexp.MustCompile(`^\{\{([a-zA-Z_][a-zA-Z0-9_]*)\}\}$`)

type RenderedCommand struct {
	Argv []string
	Line string
}

func Render(command domain.CommandSpec, args map[string]string) (RenderedCommand, error) {
	if command.Key == "" {
		return RenderedCommand{}, errors.New("command key is required")
	}
	if command.Executable == "" {
		return RenderedCommand{}, fmt.Errorf("command %q executable is required", command.Key)
	}

	argv := []string{command.Executable}
	used := map[string]bool{}
	for _, token := range command.Args {
		name, ok := templateName(token)
		if !ok {
			argv = append(argv, token)
			continue
		}
		value, exists := args[name]
		if !exists {
			return RenderedCommand{}, fmt.Errorf("argument %q is required", name)
		}
		validator, exists := command.Validators[name]
		if !exists {
			return RenderedCommand{}, fmt.Errorf("argument %q has no validator", name)
		}
		if err := Validate(name, value, validator); err != nil {
			return RenderedCommand{}, err
		}
		argv = append(argv, value)
		used[name] = true
	}
	for name := range args {
		if !used[name] {
			return RenderedCommand{}, fmt.Errorf("unexpected argument %q", name)
		}
	}
	return RenderedCommand{Argv: argv, Line: JoinPOSIX(argv)}, nil
}

func Validate(name string, value string, validator domain.Validator) error {
	if strings.ContainsRune(value, 0) {
		return fmt.Errorf("argument %q contains NUL byte", name)
	}
	if validator.MinLength > 0 && len(value) < validator.MinLength {
		return fmt.Errorf("argument %q is shorter than %d", name, validator.MinLength)
	}
	if validator.MaxLength > 0 && len(value) > validator.MaxLength {
		return fmt.Errorf("argument %q is longer than %d", name, validator.MaxLength)
	}

	switch validator.Type {
	case "enum":
		for _, allowed := range validator.Values {
			if value == allowed {
				return nil
			}
		}
		return fmt.Errorf("argument %q value %q is not allowed", name, value)
	case "regex":
		if validator.Pattern == "" {
			return fmt.Errorf("argument %q regex validator requires pattern", name)
		}
		re, err := regexp.Compile(validator.Pattern)
		if err != nil {
			return fmt.Errorf("argument %q has invalid regex: %w", name, err)
		}
		if !re.MatchString(value) {
			return fmt.Errorf("argument %q does not match required pattern", name)
		}
		return nil
	case "string":
		if value == "" {
			return fmt.Errorf("argument %q cannot be empty", name)
		}
		return nil
	default:
		return fmt.Errorf("argument %q uses unsupported validator type %q", name, validator.Type)
	}
}

func JoinPOSIX(argv []string) string {
	parts := make([]string, 0, len(argv))
	for _, arg := range argv {
		parts = append(parts, QuotePOSIX(arg))
	}
	return strings.Join(parts, " ")
}

func QuotePOSIX(value string) string {
	if value == "" {
		return "''"
	}
	if regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`).MatchString(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func templateName(value string) (string, bool) {
	matches := templatePattern.FindStringSubmatch(value)
	if len(matches) != 2 {
		return "", false
	}
	return matches[1], true
}
