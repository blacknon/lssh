package psrplib

import (
	"os"
)

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Host:                getenv(envPowerShellHost),
		User:                getenv(envPowerShellUser),
		Password:            getenv(envPowerShellPassword),
		HTTPS:               parseBoolDefault(getenv(envPowerShellUseSSL), false),
		Insecure:            parseBoolDefault(getenv(envPowerShellInsecure), false),
		Authentication:      firstNonEmpty(getenv(envPowerShellAuthentication), "Negotiate"),
		ConfigurationName:   getenv(envPowerShellConfigurationName),
		OperationTimeoutSec: parseIntDefault(getenv(envPowerShellOperationTimeout), 60),
	}
	if cfg.HTTPS {
		cfg.Port = parseIntDefault(getenv(envPowerShellPort), 5986)
	} else {
		cfg.Port = parseIntDefault(getenv(envPowerShellPort), 5985)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func getenv(key string) string {
	return os.Getenv(key)
}
