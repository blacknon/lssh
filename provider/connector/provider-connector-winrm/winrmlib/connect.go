package winrmlib

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/masterzen/winrm"
)

type TargetConfig struct {
	Host                string
	Port                int
	User                string
	Password            string
	HTTPS               bool
	Insecure            bool
	ShellCommand        string
	TLSServerName       string
	OperationTimeoutSec int
}

type Connect struct {
	cfg      TargetConfig
	endpoint *winrm.Endpoint
	client   *winrm.Client
}

func ConfigFromMaps(providerConfig, targetConfig map[string]interface{}) (TargetConfig, error) {
	cfg := TargetConfig{
		Host:                stringValue(targetConfig, "addr", stringValue(providerConfig, "addr", "")),
		User:                stringValue(targetConfig, "user", stringValue(providerConfig, "user", "")),
		Password:            stringValue(targetConfig, "pass", stringValue(providerConfig, "pass", "")),
		HTTPS:               parseBoolDefault(stringValue(targetConfig, "https", stringValue(providerConfig, "https", "")), false),
		Insecure:            parseBoolDefault(stringValue(targetConfig, "insecure", stringValue(providerConfig, "insecure", "")), false),
		ShellCommand:        strings.TrimSpace(stringValue(providerConfig, "shell_command", "")),
		TLSServerName:       strings.TrimSpace(stringValue(providerConfig, "tls_server_name", "")),
		OperationTimeoutSec: parseIntDefault(stringValue(providerConfig, "operation_timeout_sec", ""), 60),
	}
	if cfg.Host == "" {
		return TargetConfig{}, fmt.Errorf("missing target addr")
	}
	if cfg.User == "" {
		return TargetConfig{}, fmt.Errorf("missing target user")
	}
	if cfg.Password == "" {
		return TargetConfig{}, fmt.Errorf("missing target password")
	}
	rawPort := stringValue(targetConfig, "port", stringValue(providerConfig, "port", ""))
	if cfg.HTTPS {
		cfg.Port = parseIntDefault(rawPort, 5986)
	} else {
		cfg.Port = parseIntDefault(rawPort, 5985)
	}
	if cfg.ShellCommand == "" {
		cfg.ShellCommand = "cmd.exe"
	}
	return cfg, nil
}

func (c TargetConfig) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("winrm host is required")
	}
	if strings.TrimSpace(c.User) == "" {
		return fmt.Errorf("winrm user is required")
	}
	if c.Password == "" {
		return fmt.Errorf("winrm password is required")
	}
	if c.Port <= 0 {
		return fmt.Errorf("invalid winrm port %d", c.Port)
	}
	return nil
}

func CreateConnect(cfg TargetConfig) (*Connect, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	timeout := time.Duration(defaultInt(cfg.OperationTimeoutSec, 60)) * time.Second
	endpoint := winrm.NewEndpoint(cfg.Host, cfg.Port, cfg.HTTPS, cfg.Insecure, nil, nil, nil, timeout)
	endpoint.TLSServerName = cfg.TLSServerName

	client, err := winrm.NewClient(endpoint, cfg.User, cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("create winrm client: %w", err)
	}

	return &Connect{cfg: cfg, endpoint: endpoint, client: client}, nil
}

func defaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
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

func parseBoolDefault(v string, def bool) bool {
	if strings.TrimSpace(v) == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func parseIntDefault(v string, def int) int {
	if strings.TrimSpace(v) == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
