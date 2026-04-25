package psrplib

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Config struct {
	PowerShellRequired  bool
	PowerShellPath      string
	PowerShellOptions   []string
	Host                string
	User                string
	Password            string
	Port                int
	HTTPS               bool
	Insecure            bool
	Authentication      string
	ConfigurationName   string
	OperationTimeoutSec int
}

func ConfigFromMaps(providerConfig, targetConfig map[string]interface{}) (Config, error) {
	runtime := ResolveRuntime(providerConfig, targetConfig)
	cfg := Config{
		PowerShellRequired:  runtime.Shell == RuntimeCommand || runtime.Exec == RuntimeCommand,
		PowerShellPath:      firstNonEmpty(stringValue(targetConfig, "pwsh_path", ""), stringValue(providerConfig, "pwsh_path", "")),
		PowerShellOptions:   firstStringSlice(stringSliceValue(targetConfig, "pwsh_options"), stringSliceValue(providerConfig, "pwsh_options")),
		Host:                firstNonEmpty(stringValue(targetConfig, "addr", ""), stringValue(providerConfig, "addr", "")),
		User:                firstNonEmpty(stringValue(targetConfig, "user", ""), stringValue(providerConfig, "user", "")),
		Password:            firstNonEmpty(stringValue(targetConfig, "pass", ""), stringValue(providerConfig, "pass", "")),
		HTTPS:               parseBoolDefault(firstNonEmpty(stringValue(targetConfig, "https", ""), stringValue(providerConfig, "https", "")), false),
		Insecure:            parseBoolDefault(firstNonEmpty(stringValue(targetConfig, "insecure", ""), stringValue(providerConfig, "insecure", "")), false),
		Authentication:      firstNonEmpty(stringValue(targetConfig, "authentication", ""), stringValue(providerConfig, "authentication", ""), "Negotiate"),
		ConfigurationName:   firstNonEmpty(stringValue(targetConfig, "configuration_name", ""), stringValue(providerConfig, "configuration_name", "")),
		OperationTimeoutSec: parseIntDefault(firstNonEmpty(stringValue(targetConfig, "operation_timeout_sec", ""), stringValue(providerConfig, "operation_timeout_sec", "")), 60),
	}

	rawPort := firstNonEmpty(stringValue(targetConfig, "port", ""), stringValue(providerConfig, "port", ""))
	if cfg.HTTPS {
		cfg.Port = parseIntDefault(rawPort, 5986)
	} else {
		cfg.Port = parseIntDefault(rawPort, 5985)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	if cfg.PowerShellRequired {
		path, err := ResolvePowerShellPath(cfg.PowerShellPath)
		if err != nil {
			return Config{}, err
		}
		cfg.PowerShellPath = path
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("target addr is required")
	}
	if strings.TrimSpace(c.User) == "" {
		return fmt.Errorf("target user is required")
	}
	if c.Password == "" {
		return fmt.Errorf("target password is required")
	}
	if c.Port <= 0 {
		return fmt.Errorf("invalid psrp port %d", c.Port)
	}
	return nil
}

func ResolvePowerShellPath(configured string) (string, error) {
	candidates := []string{}
	if strings.TrimSpace(configured) != "" {
		candidates = append(candidates, configured)
	} else {
		candidates = append(candidates, "pwsh", "powershell", "powershell.exe")
	}

	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}

	if strings.TrimSpace(configured) != "" {
		return "", fmt.Errorf("powershell executable %q was not found", configured)
	}
	return "", fmt.Errorf("no PowerShell executable was found; set pwsh_path explicitly")
}

func stringValue(raw map[string]interface{}, key, def string) string {
	if raw == nil {
		return def
	}
	if value, ok := raw[key]; ok && value != nil {
		return fmt.Sprint(value)
	}
	return def
}

func stringSliceValue(raw map[string]interface{}, key string) []string {
	if raw == nil {
		return nil
	}
	value, ok := raw[key]
	if !ok || value == nil {
		return nil
	}

	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if item == nil {
				continue
			}
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return []string{fmt.Sprint(typed)}
	}
}

func parseBoolDefault(v string, def bool) bool {
	if strings.TrimSpace(v) == "" {
		return def
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return parsed
}

func parseIntDefault(v string, def int) int {
	if strings.TrimSpace(v) == "" {
		return def
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstStringSlice(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return append([]string(nil), value...)
		}
	}
	return nil
}
