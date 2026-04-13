package providerbuiltin

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/blacknon/lssh/internal/providerapi"
)

func ReadRequest() (providerapi.Request, error) {
	var req providerapi.Request
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return req, err
	}
	err = json.Unmarshal(data, &req)
	return req, err
}

func WriteResult(v interface{}) error {
	data, err := json.Marshal(struct {
		Version string      `json:"version"`
		Result  interface{} `json:"result"`
	}{
		Version: providerapi.Version,
		Result:  v,
	})
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

func WriteError(message string) error {
	data, err := json.Marshal(struct {
		Version string             `json:"version"`
		Error   *providerapi.Error `json:"error"`
	}{
		Version: providerapi.Version,
		Error:   &providerapi.Error{Message: message},
	})
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

func String(raw map[string]interface{}, key string) string {
	if raw == nil {
		return ""
	}
	value, ok := raw[key]
	if !ok {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func StringSlice(raw map[string]interface{}, key string) []string {
	if raw == nil {
		return nil
	}
	value, ok := raw[key]
	if !ok {
		return nil
	}
	switch v := value.(type) {
	case []string:
		return append([]string{}, v...)
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return nil
	}
}

func Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	return output, nil
}

func RunWithEnv(env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	return output, nil
}
