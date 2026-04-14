package providerbuiltin

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

func ResolveConfigValue(raw map[string]interface{}, key string) (string, error) {
	if value := String(raw, key); value != "" {
		return value, nil
	}

	if envName := String(raw, key+"_env"); envName != "" {
		value := os.Getenv(envName)
		if value == "" {
			return "", fmt.Errorf("%s is set but environment variable %q is empty", key+"_env", envName)
		}
		return value, nil
	}

	if sourcePath := String(raw, key+"_source"); sourcePath != "" {
		envName := String(raw, key+"_source_env")
		if envName == "" {
			envName = strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
		}
		values, err := readEnvSourceFile(sourcePath)
		if err != nil {
			return "", err
		}
		value := values[envName]
		if value == "" {
			return "", fmt.Errorf("%s did not define %q", key+"_source", envName)
		}
		return value, nil
	}

	return "", nil
}

func readEnvSourceFile(path string) (map[string]string, error) {
	fullPath := expandPath(path)
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return path
}
