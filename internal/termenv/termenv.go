package termenv

import (
	"fmt"
	"os"
	"strings"
)

const DefaultTerm = "xterm-256color"

func Current() string {
	term := strings.TrimSpace(os.Getenv("TERM"))
	if term == "" {
		return DefaultTerm
	}

	return term
}

func MergeEnv(overrides map[string]string) []string {
	result := append([]string{}, os.Environ()...)
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}

	return Ensure(result)
}

func Ensure(env []string) []string {
	for i, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key != "TERM" {
			continue
		}
		if strings.TrimSpace(value) != "" {
			return env
		}
		env[i] = "TERM=" + Current()
		return env
	}

	return append(env, "TERM="+Current())
}

func WrapShellExec(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}

	return fmt.Sprintf("export TERM=%s; exec %s", shellSingleQuote(Current()), command)
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
