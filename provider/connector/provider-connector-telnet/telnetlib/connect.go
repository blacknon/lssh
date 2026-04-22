package telnetlib

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

type TargetConfig struct {
	Host             string
	Port             int
	Username         string
	Password         string
	DialTimeoutSec   int
	LoginPrompt      string
	PasswordPrompt   string
	InitialNewline   bool
	PostLoginDelayMs int
}

type Connect struct {
	cfg  TargetConfig
	conn net.Conn
}

func ConfigFromMaps(providerConfig, targetConfig map[string]interface{}) (TargetConfig, error) {
	cfg := TargetConfig{
		Host:             stringValue(targetConfig, "addr", stringValue(providerConfig, "addr", "")),
		Username:         stringValue(targetConfig, "user", stringValue(providerConfig, "user", "")),
		Password:         stringValue(targetConfig, "pass", stringValue(providerConfig, "pass", "")),
		LoginPrompt:      stringValue(providerConfig, "login_prompt", "login:"),
		PasswordPrompt:   stringValue(providerConfig, "password_prompt", "password:"),
		InitialNewline:   parseBoolDefault(stringValue(providerConfig, "initial_newline", ""), true),
		DialTimeoutSec:   parseIntDefault(stringValue(providerConfig, "dial_timeout_sec", ""), 10),
		PostLoginDelayMs: parseIntDefault(stringValue(providerConfig, "post_login_delay_ms", ""), 200),
	}
	cfg.Port = parseIntDefault(stringValue(targetConfig, "port", stringValue(providerConfig, "port", "")), 23)
	if err := cfg.Validate(); err != nil {
		return TargetConfig{}, err
	}
	return cfg, nil
}

func (c TargetConfig) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("telnet host is required")
	}
	if c.Port <= 0 {
		return fmt.Errorf("invalid telnet port %d", c.Port)
	}
	return nil
}

func Dial(cfg TargetConfig) (*Connect, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	timeout := time.Duration(defaultInt(cfg.DialTimeoutSec, 10)) * time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), timeout)
	if err != nil {
		return nil, fmt.Errorf("dial telnet: %w", err)
	}
	return &Connect{cfg: cfg, conn: conn}, nil
}

func ConfigFromPlanDetails(details map[string]interface{}) (TargetConfig, error) {
	cfg := TargetConfig{
		Host:             stringValue(details, "addr", ""),
		Username:         stringValue(details, "user", ""),
		Password:         stringValue(details, "pass", ""),
		LoginPrompt:      stringValue(details, "login_prompt", "login:"),
		PasswordPrompt:   stringValue(details, "password_prompt", "password:"),
		InitialNewline:   parseBoolDefault(stringValue(details, "initial_newline", ""), true),
		DialTimeoutSec:   parseIntDefault(stringValue(details, "dial_timeout_sec", ""), 10),
		PostLoginDelayMs: parseIntDefault(stringValue(details, "post_login_delay_ms", ""), 200),
	}
	cfg.Port = parseIntDefault(stringValue(details, "port", ""), 23)
	if err := cfg.Validate(); err != nil {
		return TargetConfig{}, err
	}
	return cfg, nil
}

func (c *Connect) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
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
