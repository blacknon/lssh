package conf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/blacknon/lssh/internal/providerapi"
)

func mergeProvidersConfig(base, override ProvidersConfig) ProvidersConfig {
	result := base
	if len(override.Paths) > 0 {
		result.Paths = append(append([]string{}, result.Paths...), override.Paths...)
	}
	if override.Timeout != "" {
		result.Timeout = override.Timeout
	}
	if override.InventoryCacheTTL != "" {
		result.InventoryCacheTTL = override.InventoryCacheTTL
	}
	if override.FailOpen {
		result.FailOpen = true
	}
	return result
}

func (c *Config) ReadInventoryProviders() error {
	if len(c.Provider) == 0 {
		return nil
	}

	names := make([]string, 0, len(c.Provider))
	for name := range c.Provider {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		raw := c.Provider[name]
		if !providerEnabled(raw) || !providerHasCapability(raw, "inventory") {
			continue
		}

		var result providerapi.InventoryListResult
		if err := c.callProvider(name, providerapi.MethodInventoryList, providerapi.InventoryListParams{
			Provider: name,
			Config:   raw,
		}, &result); err != nil {
			if providerFailOpen(c.Providers, raw) {
				continue
			}
			return err
		}

		defaults, err := providerServerDefaults(raw)
		if err != nil {
			return fmt.Errorf("provider %q defaults: %w", name, err)
		}
		matches, err := providerInventoryMatches(raw)
		if err != nil {
			return fmt.Errorf("provider %q matches: %w", name, err)
		}
		base := serverConfigReduct(c.Common, defaults)

		for _, server := range result.Servers {
			if server.Name == "" {
				return fmt.Errorf("provider %q returned an empty server name", name)
			}

			var generated ServerConfig
			if err := decodeTaggedMap(server.Config, &generated, "toml"); err != nil {
				return fmt.Errorf("provider %q server %q: %w", name, server.Name, err)
			}

			merged := serverConfigReduct(base, generated)
			merged = applyProviderInventoryMatches(name, server.Name, server.Meta, merged, matches)
			c.Server[server.Name] = merged
		}
	}

	return nil
}

func (c *Config) ResolveSecretRef(ref, server, field string) (string, error) {
	providerName, secretRef, err := parseSecretRef(ref)
	if err != nil {
		return "", err
	}

	raw, ok := c.Provider[providerName]
	if !ok {
		return "", fmt.Errorf("provider %q is not configured", providerName)
	}
	if !providerEnabled(raw) {
		return "", fmt.Errorf("provider %q is disabled", providerName)
	}
	if !providerHasCapability(raw, "secret") {
		return "", fmt.Errorf("provider %q does not support secret capability", providerName)
	}

	var result providerapi.SecretGetResult
	if err := c.callProvider(providerName, providerapi.MethodSecretGet, providerapi.SecretGetParams{
		Provider: providerName,
		Config:   raw,
		Ref:      secretRef,
		Server:   server,
		Field:    field,
	}, &result); err != nil {
		return "", err
	}

	return result.Value, nil
}

func (c *Config) callProvider(name, method string, params interface{}, out interface{}) error {
	raw, ok := c.Provider[name]
	if !ok {
		return fmt.Errorf("provider %q is not configured", name)
	}

	pluginName := providerString(raw, "plugin")
	if pluginName == "" {
		pluginName = name
	}

	path, err := resolveProviderExecutable(c.Providers, pluginName)
	if err != nil {
		return fmt.Errorf("provider %q: %w", name, err)
	}

	timeout := providerTimeout(c.Providers, raw)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req := providerapi.Request{
		Version: providerapi.Version,
		Method:  method,
		Params:  params,
	}

	input, err := json.Marshal(req)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = bytes.NewReader(input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return err
	}

	var resp providerapi.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return fmt.Errorf("decode response from %q: %w", path, err)
	}
	if resp.Error != nil {
		return fmt.Errorf("%s", resp.Error.Message)
	}
	if out == nil || len(resp.Result) == 0 {
		return nil
	}
	return json.Unmarshal(resp.Result, out)
}

func providerServerDefaults(raw map[string]interface{}) (ServerConfig, error) {
	cfg := ServerConfig{}
	if err := decodeTaggedMap(raw, &cfg, "toml"); err != nil {
		return cfg, err
	}
	return cfg, nil
}

type providerInventoryMatchWhen struct {
	NameIn        []string
	NameNotIn     []string
	ProviderIn    []string
	ProviderNotIn []string
	MetaIn        []string
	MetaNotIn     []string
}

type providerInventoryMatch struct {
	Name     string
	Priority int
	Config   ServerConfig
	When     providerInventoryMatchWhen
	order    int
}

func providerInventoryMatches(raw map[string]interface{}) ([]providerInventoryMatch, error) {
	matchRaw, ok := raw["match"]
	if !ok {
		return nil, nil
	}

	matchMap, ok := matchRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("match must be a table")
	}

	names := make([]string, 0, len(matchMap))
	for name := range matchMap {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]providerInventoryMatch, 0, len(names))
	for idx, name := range names {
		branchMap, ok := matchMap[name].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("match.%s must be a table", name)
		}

		var cfg ServerConfig
		if err := decodeTaggedMap(branchMap, &cfg, "toml"); err != nil {
			return nil, err
		}

		when, err := decodeProviderInventoryWhen(branchMap)
		if err != nil {
			return nil, fmt.Errorf("match.%s: %w", name, err)
		}

		priority := 100
		if rawPriority, ok := branchMap["priority"]; ok {
			if p, ok := asInt64(rawPriority); ok {
				priority = int(p)
			} else {
				return nil, fmt.Errorf("match.%s.priority must be an integer", name)
			}
		}

		result = append(result, providerInventoryMatch{
			Name:     name,
			Priority: priority,
			Config:   cfg,
			When:     when,
			order:    idx + 1,
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].order < result[j].order
	})

	return result, nil
}

func decodeProviderInventoryWhen(raw interface{}) (providerInventoryMatchWhen, error) {
	when := providerInventoryMatchWhen{}
	if raw == nil {
		return when, nil
	}

	table, ok := raw.(map[string]interface{})
	if !ok {
		return when, fmt.Errorf("must be a table")
	}

	when.NameIn = providerStringSlice(table, "name_in")
	when.NameNotIn = providerStringSlice(table, "name_not_in")
	when.ProviderIn = providerStringSlice(table, "provider_in")
	when.ProviderNotIn = providerStringSlice(table, "provider_not_in")
	when.MetaIn = providerStringSlice(table, "meta_in")
	when.MetaNotIn = providerStringSlice(table, "meta_not_in")

	// Backward-compatible fallback for the older nested form.
	if nested, ok := table["when"]; ok {
		nestedWhen, err := decodeProviderInventoryWhen(nested)
		if err != nil {
			return when, err
		}
		if len(when.NameIn) == 0 {
			when.NameIn = nestedWhen.NameIn
		}
		if len(when.NameNotIn) == 0 {
			when.NameNotIn = nestedWhen.NameNotIn
		}
		if len(when.ProviderIn) == 0 {
			when.ProviderIn = nestedWhen.ProviderIn
		}
		if len(when.ProviderNotIn) == 0 {
			when.ProviderNotIn = nestedWhen.ProviderNotIn
		}
		if len(when.MetaIn) == 0 {
			when.MetaIn = nestedWhen.MetaIn
		}
		if len(when.MetaNotIn) == 0 {
			when.MetaNotIn = nestedWhen.MetaNotIn
		}
	}

	return when, nil
}

func applyProviderInventoryMatches(providerName, serverName string, meta map[string]string, base ServerConfig, matches []providerInventoryMatch) ServerConfig {
	current := base
	for _, match := range matches {
		if providerInventoryMatchApplies(match.When, providerName, serverName, meta) {
			current = serverConfigReduct(current, match.Config)
		}
	}
	return current
}

func providerInventoryMatchApplies(when providerInventoryMatchWhen, providerName, serverName string, meta map[string]string) bool {
	if !matchPatternList(serverName, when.NameIn, false) {
		return false
	}
	if !matchPatternList(serverName, when.NameNotIn, true) {
		return false
	}
	if !matchPatternList(providerName, when.ProviderIn, false) {
		return false
	}
	if !matchPatternList(providerName, when.ProviderNotIn, true) {
		return false
	}
	if !matchMetaRules(meta, when.MetaIn, false) {
		return false
	}
	if !matchMetaRules(meta, when.MetaNotIn, true) {
		return false
	}
	return true
}

func matchPatternList(value string, patterns []string, negative bool) bool {
	if len(patterns) == 0 {
		return true
	}
	matched := false
	for _, pattern := range patterns {
		ok, err := path.Match(pattern, value)
		if err == nil && ok {
			matched = true
			break
		}
	}
	if negative {
		return !matched
	}
	return matched
}

func matchMetaRules(meta map[string]string, rules []string, negative bool) bool {
	if len(rules) == 0 {
		return true
	}

	matched := false
	for _, rule := range rules {
		parts := strings.SplitN(rule, "=", 2)
		if len(parts) != 2 {
			continue
		}
		value := meta[parts[0]]
		ok, err := path.Match(parts[1], value)
		if err == nil && ok {
			matched = true
			break
		}
	}
	if negative {
		return !matched
	}
	return matched
}

func providerEnabled(raw map[string]interface{}) bool {
	v, ok := raw["enabled"]
	if !ok {
		return true
	}
	b, ok := asBool(v)
	return ok && b
}

func providerHasCapability(raw map[string]interface{}, capability string) bool {
	values := providerStringSlice(raw, "capabilities")
	if len(values) == 0 {
		return false
	}
	for _, v := range values {
		if v == capability {
			return true
		}
	}
	return false
}

func providerFailOpen(global ProvidersConfig, raw map[string]interface{}) bool {
	if v, ok := raw["fail_open"]; ok {
		if b, ok := asBool(v); ok {
			return b
		}
	}
	return global.FailOpen
}

func providerTimeout(global ProvidersConfig, raw map[string]interface{}) time.Duration {
	value := providerString(raw, "timeout")
	if value == "" {
		value = global.Timeout
	}
	if value == "" {
		return 5 * time.Second
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 5 * time.Second
	}
	return d
}

func providerString(raw map[string]interface{}, key string) string {
	if raw == nil {
		return ""
	}
	v, ok := raw[key]
	if !ok {
		return ""
	}
	switch value := v.(type) {
	case string:
		return value
	default:
		return fmt.Sprint(value)
	}
}

func providerStringSlice(raw map[string]interface{}, key string) []string {
	if raw == nil {
		return nil
	}
	v, ok := raw[key]
	if !ok {
		return nil
	}
	switch value := v.(type) {
	case []string:
		return append([]string{}, value...)
	case []interface{}:
		out := make([]string, 0, len(value))
		for _, item := range value {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return nil
	}
}

func parseSecretRef(ref string) (string, string, error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid provider secret ref %q", ref)
	}
	return parts[0], parts[1], nil
}

func resolveProviderExecutable(global ProvidersConfig, plugin string) (string, error) {
	candidates := []string{plugin, "lssh-provider-" + plugin}
	paths := providerSearchPaths(global)
	if strings.Contains(plugin, string(filepath.Separator)) {
		full := expandProviderPath(plugin)
		for _, base := range paths {
			if sameFileOrWithin(full, base) && isExecutableFile(full) {
				return full, nil
			}
		}
		return "", fmt.Errorf("provider executable %q is outside configured provider paths", plugin)
	}

	for _, entry := range paths {
		if isExecutableFile(entry) {
			base := filepath.Base(entry)
			for _, candidate := range candidates {
				if base == candidate {
					return entry, nil
				}
			}
			continue
		}

		for _, candidate := range candidates {
			full := filepath.Join(entry, candidate)
			if isExecutableFile(full) {
				return full, nil
			}
		}
	}

	return "", fmt.Errorf("provider executable %q was not found in configured provider paths", plugin)
}

func providerSearchPaths(global ProvidersConfig) []string {
	paths := make([]string, 0, len(global.Paths)+2)
	for _, path := range global.Paths {
		paths = append(paths, expandProviderPath(path))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths,
			filepath.Join(home, ".config", "lssh", "providers"),
			filepath.Join(home, ".local", "share", "lssh", "providers"),
		)
	}
	return uniquePaths(paths)
}

func expandProviderPath(path string) string {
	if path == "" {
		return ""
	}
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			path = home
		}
	} else if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	full, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(full)
}

func uniquePaths(paths []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func pathWithin(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func sameFileOrWithin(path, base string) bool {
	if path == base {
		return true
	}
	if isExecutableFile(base) {
		return false
	}
	return pathWithin(path, base)
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

func decodeTaggedMap(raw map[string]interface{}, out interface{}, tagName string) error {
	value := reflect.ValueOf(out)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return fmt.Errorf("output must be a non-nil pointer")
	}

	structValue := value.Elem()
	structType := structValue.Type()
	for i := 0; i < structType.NumField(); i++ {
		fieldType := structType.Field(i)
		tag := fieldType.Tag.Get(tagName)
		if tag == "" || tag == "-" {
			continue
		}
		rawValue, ok := raw[tag]
		if !ok {
			continue
		}
		if err := setTaggedValue(structValue.Field(i), rawValue); err != nil {
			return fmt.Errorf("set %s: %w", tag, err)
		}
	}

	return nil
}

func setTaggedValue(field reflect.Value, raw interface{}) error {
	if !field.CanSet() {
		return nil
	}

	switch field.Interface().(type) {
	case ControlPersistDuration:
		parsed, err := parseControlPersist(raw)
		if err != nil {
			return err
		}
		field.SetInt(int64(parsed))
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(fmt.Sprint(raw))
		return nil
	case reflect.Bool:
		v, ok := asBool(raw)
		if !ok {
			return fmt.Errorf("expected bool, got %T", raw)
		}
		field.SetBool(v)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v, ok := asInt64(raw)
		if !ok {
			return fmt.Errorf("expected integer, got %T", raw)
		}
		field.SetInt(v)
		return nil
	case reflect.Slice:
		if field.Type().Elem().Kind() != reflect.String {
			return nil
		}
		values := providerStringSlice(map[string]interface{}{"_": raw}, "_")
		field.Set(reflect.ValueOf(values))
		return nil
	default:
		return nil
	}
}

func asBool(v interface{}) (bool, bool) {
	switch value := v.(type) {
	case bool:
		return value, true
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "yes", "1":
			return true, true
		case "false", "no", "0":
			return false, true
		}
	}
	return false, false
}

func asInt64(v interface{}) (int64, bool) {
	switch value := v.(type) {
	case int:
		return int64(value), true
	case int8:
		return int64(value), true
	case int16:
		return int64(value), true
	case int32:
		return int64(value), true
	case int64:
		return value, true
	case float64:
		return int64(value), true
	}
	return 0, false
}
