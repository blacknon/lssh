package conf

import (
	"fmt"
	"strconv"
	"time"
)

// ControlPersistDuration stores ControlPersist in seconds while allowing TOML/OpenSSH duration strings.
type ControlPersistDuration int

func (d *ControlPersistDuration) UnmarshalTOML(v interface{}) error {
	parsed, err := parseControlPersist(v)
	if err != nil {
		return err
	}

	*d = parsed
	return nil
}

func parseControlPersist(v interface{}) (ControlPersistDuration, error) {
	switch value := v.(type) {
	case int64:
		return ControlPersistDuration(value), nil
	case int:
		return ControlPersistDuration(value), nil
	case string:
		if value == "" {
			return 0, nil
		}

		if i, err := strconv.Atoi(value); err == nil {
			return ControlPersistDuration(i), nil
		}

		d, err := time.ParseDuration(value)
		if err != nil {
			return 0, fmt.Errorf("invalid control_persist value %q: %w", value, err)
		}

		return ControlPersistDuration(d / time.Second), nil
	default:
		return 0, fmt.Errorf("invalid control_persist type %T", v)
	}
}
