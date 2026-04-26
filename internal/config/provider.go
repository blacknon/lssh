package conf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/blacknon/lssh/providerapi"
)

func mergeProvidersConfig(base, override ProvidersConfig) ProvidersConfig {
	result := base
	if len(override.Paths) > 0 {
		result.Paths = append(append([]string{}, result.Paths...), override.Paths...)
	}
	if override.Timeout != "" {
		result.Timeout = override.Timeout
	}
	if override.MaxParallel > 0 {
		result.MaxParallel = override.MaxParallel
	}
	if override.InventoryCacheTTL != "" {
		result.InventoryCacheTTL = override.InventoryCacheTTL
	}
	if override.FailOpen {
		result.FailOpen = true
	}
	if override.DebugLog != "" {
		result.DebugLog = override.DebugLog
	}
	return result
}

func (c *Config) ReadInventoryProviders() error {
	if len(c.Provider) == 0 {
		return nil
	}

	providers := c.activeProviders()
	results := c.fetchInventoryProviderResults(providers)

	for i, item := range providers {
		result := results[i]
		if result.skip {
			continue
		}
		if result.err != nil {
			if providerFailOpen(c.Providers, result.raw) {
				log.Printf("provider %q inventory failed but fail_open=true: %v", item.name, result.err)
				continue
			}
			return result.err
		}

		rawWithMeta := mergeProviderDescribeMetadata(result.raw, result.describe)

		defaults, err := providerServerDefaults(rawWithMeta)
		if err != nil {
			return fmt.Errorf("provider %q defaults: %w", item.name, err)
		}
		matches, err := providerInventoryMatches(rawWithMeta)
		if err != nil {
			return fmt.Errorf("provider %q matches: %w", item.name, err)
		}
		base := serverConfigReduct(c.Common, defaults)

		for _, server := range result.inventory.Servers {
			if server.Name == "" {
				return fmt.Errorf("provider %q returned an empty server name", item.name)
			}

			var generated ServerConfig
			if err := decodeTaggedMap(server.Config, &generated, "toml"); err != nil {
				return fmt.Errorf("provider %q server %q: %w", item.name, server.Name, err)
			}

			merged := serverConfigReduct(base, generated)
			merged = applyProviderInventoryMatches(item.name, server.Name, server.Meta, merged, matches)
			merged.ProviderName = item.name
			merged.ProviderPlugin = providerString(result.raw, "plugin")
			merged.ProviderMeta = cloneProviderMeta(server.Meta)
			c.Server[server.Name] = merged
		}
	}

	return nil
}

type inventoryProviderResult struct {
	raw       map[string]interface{}
	describe  providerapi.PluginDescribeResult
	inventory providerapi.InventoryListResult
	err       error
	skip      bool
}

func (c *Config) fetchInventoryProviderResults(providers []namedProviderConfig) []inventoryProviderResult {
	results := make([]inventoryProviderResult, len(providers))
	var wg sync.WaitGroup
	semaphore := providerInventorySemaphore(c.Providers)

	for i, item := range providers {
		i := i
		name := item.name
		raw := c.Provider[name]
		if !providerEnabled(raw) || !providerHasCapability(raw, "inventory") {
			results[i] = inventoryProviderResult{raw: raw, skip: true}
			continue
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			if semaphore != nil {
				semaphore <- struct{}{}
				defer func() {
					<-semaphore
				}()
			}

			results[i].raw = raw
			if providerNeedsDescribeReservedKeys(raw) {
				_ = c.callProvider(name, providerapi.MethodPluginDescribe, nil, &results[i].describe)
			}
			err := c.callProvider(name, providerapi.MethodInventoryList, providerapi.InventoryListParams{
				Provider: name,
				Config:   raw,
			}, &results[i].inventory)
			if err != nil {
				results[i].err = err
			}
		}()
	}

	wg.Wait()
	return results
}

func providerInventorySemaphore(global ProvidersConfig) chan struct{} {
	if global.MaxParallel <= 0 {
		return nil
	}

	return make(chan struct{}, global.MaxParallel)
}

type namedProviderConfig struct {
	name string
}

func (c *Config) activeProviders() []namedProviderConfig {
	if len(c.Provider) == 0 {
		return nil
	}

	reqs, err := c.validateProviderWhens()
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	ctx := matchContext{}
	if reqs.needLocalIP || reqs.needGateway || reqs.needUsername || reqs.needHostname || reqs.needOS || reqs.needTerm || reqs.needEnv {
		ctx = detectMatchContext(reqs)
	}

	names := make([]string, 0, len(c.Provider))
	for name := range c.Provider {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]namedProviderConfig, 0, len(names))
	for _, name := range names {
		raw := c.Provider[name]
		when, err := providerWhen(raw)
		if err != nil {
			log.Printf("provider.%s.when: %v", name, err)
			os.Exit(1)
		}
		if when.Empty() || whenMatches(when, "provider", name, ctx) {
			result = append(result, namedProviderConfig{name: name})
		}
	}

	return result
}

func (c *Config) validateProviderWhens() (matchRequirements, error) {
	reqs := matchRequirements{}

	for name, raw := range c.Provider {
		when, err := providerWhen(raw)
		if err != nil {
			return reqs, fmt.Errorf("provider.%s.when: %w", name, err)
		}
		if when.Empty() {
			continue
		}

		if err := validateMatchNetworkList(when.LocalIPIn, "local_ip_in", "provider", name); err != nil {
			return reqs, err
		}
		if err := validateMatchNetworkList(when.LocalIPNotIn, "local_ip_not_in", "provider", name); err != nil {
			return reqs, err
		}
		if err := validateMatchNetworkList(when.GatewayIn, "gateway_in", "provider", name); err != nil {
			return reqs, err
		}
		if err := validateMatchNetworkList(when.GatewayNotIn, "gateway_not_in", "provider", name); err != nil {
			return reqs, err
		}

		reqs.needLocalIP = reqs.needLocalIP || len(when.LocalIPIn) > 0 || len(when.LocalIPNotIn) > 0
		reqs.needGateway = reqs.needGateway || len(when.GatewayIn) > 0 || len(when.GatewayNotIn) > 0
		reqs.needUsername = reqs.needUsername || len(when.UsernameIn) > 0 || len(when.UsernameNotIn) > 0
		reqs.needHostname = reqs.needHostname || len(when.HostnameIn) > 0 || len(when.HostnameNotIn) > 0
		reqs.needOS = reqs.needOS || len(when.OSIn) > 0 || len(when.OSNotIn) > 0
		reqs.needTerm = reqs.needTerm || len(when.TermIn) > 0 || len(when.TermNotIn) > 0
		reqs.needEnv = reqs.needEnv || len(when.EnvIn) > 0 || len(when.EnvNotIn) > 0 || len(when.EnvValueIn) > 0 || len(when.EnvValueNotIn) > 0
	}

	return reqs, nil
}

func providerWhen(raw map[string]interface{}) (ServerMatchWhen, error) {
	when := ServerMatchWhen{}
	if raw == nil {
		return when, nil
	}
	rawWhen, ok := raw["when"]
	if !ok || rawWhen == nil {
		return when, nil
	}
	whenMap, ok := rawWhen.(map[string]interface{})
	if !ok {
		return when, fmt.Errorf("must be a table")
	}
	if err := decodeTaggedMap(whenMap, &when, "toml"); err != nil {
		return when, err
	}
	return when, nil
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
	debugLogPath := providerDebugLogPath(c.Providers, raw)
	sensitiveValues := collectProviderSensitiveValues(method, input, nil)

	if err := cmd.Run(); err != nil {
		sensitiveValues = collectProviderSensitiveValues(method, input, stdout.Bytes())
		writeProviderDebugLog(debugLogPath, name, method, path, input, stdout.Bytes(), stderr.Bytes(), err)
		if stdout.Len() > 0 {
			var resp providerapi.Response
			if decodeErr := json.Unmarshal(stdout.Bytes(), &resp); decodeErr == nil && resp.Error != nil {
				return &ProviderError{
					Provider: name,
					Code:     resp.Error.Code,
					Message:  sanitizeProviderDebugString(resp.Error.Message, sensitiveValues),
				}
			}
		}
		if stderr.Len() > 0 {
			return fmt.Errorf("provider %q: %w: %s", name, err, sanitizeProviderDebugString(strings.TrimSpace(stderr.String()), sensitiveValues))
		}
		return fmt.Errorf("provider %q: %w", name, err)
	}
	writeProviderDebugLog(debugLogPath, name, method, path, input, stdout.Bytes(), stderr.Bytes(), nil)
	sensitiveValues = collectProviderSensitiveValues(method, input, stdout.Bytes())

	var resp providerapi.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return fmt.Errorf("provider %q decode response from %q: %w", name, path, err)
	}
	if resp.Error != nil {
		return &ProviderError{
			Provider: name,
			Code:     resp.Error.Code,
			Message:  sanitizeProviderDebugString(resp.Error.Message, sensitiveValues),
		}
	}
	if out == nil || len(resp.Result) == 0 {
		return nil
	}
	return json.Unmarshal(resp.Result, out)
}

func providerServerDefaults(raw map[string]interface{}) (ServerConfig, error) {
	cfg := ServerConfig{}
	if err := decodeTaggedMap(providerServerDefaultMap(raw), &cfg, "toml"); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func providerServerDefaultMap(raw map[string]interface{}) map[string]interface{} {
	if raw == nil {
		return nil
	}

	filtered := make(map[string]interface{}, len(raw))
	for key, value := range raw {
		filtered[key] = value
	}

	for _, key := range providerReservedKeys(raw) {
		delete(filtered, key)
	}

	return filtered
}

func mergeProviderDescribeMetadata(raw map[string]interface{}, describe providerapi.PluginDescribeResult) map[string]interface{} {
	if raw == nil && len(describe.ReservedKeys) == 0 {
		return nil
	}

	merged := make(map[string]interface{}, len(raw)+1)
	for key, value := range raw {
		merged[key] = value
	}
	if len(describe.ReservedKeys) > 0 {
		values := make([]interface{}, 0, len(describe.ReservedKeys))
		for _, key := range describe.ReservedKeys {
			values = append(values, key)
		}
		merged["reserved_keys"] = values
	}
	return merged
}

func providerReservedKeys(raw map[string]interface{}) []string {
	keys := map[string]struct{}{
		"plugin":                 {},
		"capabilities":           {},
		"default_connector_name": {},
		"enabled":                {},
		"fail_open":              {},
		"timeout":                {},
		"debug_log":              {},
		"match":                  {},
	}

	for _, key := range providerStringSlice(raw, "reserved_keys") {
		keys[key] = struct{}{}
	}

	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func providerNeedsDescribeReservedKeys(raw map[string]interface{}) bool {
	if raw == nil {
		return false
	}
	if len(providerStringSlice(raw, "reserved_keys")) > 0 {
		return false
	}
	for key := range raw {
		switch key {
		case "plugin", "capabilities", "default_connector_name", "enabled", "fail_open", "timeout", "debug_log", "match":
			continue
		default:
			return true
		}
	}
	return false
}

func providerExecutableCandidates(plugin string) []string {
	names := []string{plugin}
	switch {
	case strings.HasPrefix(plugin, "provider-mixed-"):
		names = append(names, strings.Replace(plugin, "provider-mixed-", "provider-inventory-", 1))
	case strings.HasPrefix(plugin, "provider-inventory-"):
		names = append(names, strings.Replace(plugin, "provider-inventory-", "provider-mixed-", 1))
	}

	seen := map[string]struct{}{}
	candidates := make([]string, 0, len(names)*2)
	for _, name := range names {
		if name == "" {
			continue
		}
		for _, candidate := range []string{name, "lssh-provider-" + name} {
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

type providerInventoryMatchWhen struct {
	NameIn        []string
	NameNotIn     []string
	ProviderIn    []string
	ProviderNotIn []string
	MetaIn        []string
	MetaNotIn     []string
	MetaAllIn     []string
	MetaAllNotIn  []string
}

type providerInventoryMatch struct {
	Name         string
	Priority     int
	Config       ServerConfig
	ExtraConfig  map[string]interface{}
	When         providerInventoryMatchWhen
	NoteTemplate string
	NoteAppend   string
	order        int
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

	result := make([]providerInventoryMatch, 0, len(matchMap))
	for name, branch := range matchMap {
		branchMap, ok := branch.(map[string]interface{})
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
			Name:         name,
			Priority:     priority,
			Config:       cfg,
			ExtraConfig:  providerInventoryMatchExtraConfig(branchMap),
			When:         when,
			NoteTemplate: providerString(branchMap, "note_template"),
			NoteAppend:   providerString(branchMap, "note_append"),
			order:        providerMatchOrder(branchMap),
		})
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		if result[i].order != result[j].order {
			return result[i].order < result[j].order
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}

func providerMatchOrder(branchMap map[string]interface{}) int {
	if branchMap == nil {
		return 0
	}
	if raw, ok := branchMap[providerMatchOrderKey]; ok {
		if value, ok := asInt64(raw); ok {
			return int(value)
		}
	}
	return 0
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
	when.MetaAllIn = providerStringSlice(table, "meta_all_in")
	when.MetaAllNotIn = providerStringSlice(table, "meta_all_not_in")

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
		if len(when.MetaAllIn) == 0 {
			when.MetaAllIn = nestedWhen.MetaAllIn
		}
		if len(when.MetaAllNotIn) == 0 {
			when.MetaAllNotIn = nestedWhen.MetaAllNotIn
		}
	}

	return when, nil
}

func applyProviderInventoryMatches(providerName, serverName string, meta map[string]string, base ServerConfig, matches []providerInventoryMatch) ServerConfig {
	current := base
	for _, match := range matches {
		if providerInventoryMatchApplies(match.When, providerName, serverName, meta) {
			current = serverConfigReduct(current, match.Config)
			current.ProviderConfig = mergeProviderConfigMaps(current.ProviderConfig, match.ExtraConfig)
			current.Note = applyProviderInventoryNoteTemplate(current.Note, providerName, serverName, meta, match)
		}
	}
	return current
}

func providerInventoryMatchExtraConfig(branchMap map[string]interface{}) map[string]interface{} {
	if branchMap == nil {
		return nil
	}

	controlKeys := map[string]struct{}{
		"name_in":         {},
		"name_not_in":     {},
		"provider_in":     {},
		"provider_not_in": {},
		"meta_in":         {},
		"meta_not_in":     {},
		"meta_all_in":     {},
		"meta_all_not_in": {},
		"note_template":   {},
		"note_append":     {},
		"priority":        {},
		"when":            {},
	}
	for _, key := range serverConfigTOMLKeys() {
		controlKeys[key] = struct{}{}
	}

	result := map[string]interface{}{}
	for key, value := range branchMap {
		if _, ok := controlKeys[key]; ok {
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func applyProviderInventoryNoteTemplate(currentNote, providerName, serverName string, meta map[string]string, match providerInventoryMatch) string {
	if match.NoteTemplate != "" {
		currentNote = renderProviderInventoryNoteTemplate(match.NoteTemplate, currentNote, providerName, serverName, meta)
	}
	if match.NoteAppend != "" {
		currentNote += renderProviderInventoryNoteTemplate(match.NoteAppend, currentNote, providerName, serverName, meta)
	}
	return currentNote
}

func renderProviderInventoryNoteTemplate(template, currentNote, providerName, serverName string, meta map[string]string) string {
	if template == "" {
		return ""
	}

	result := template
	replacements := []string{
		"${note}", currentNote,
		"${provider}", providerName,
		"${server}", serverName,
	}
	for key, value := range meta {
		replacements = append(replacements, "${meta:"+key+"}", value)
	}
	return strings.NewReplacer(replacements...).Replace(result)
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
	if !matchMetaRulesAll(meta, when.MetaAllIn, false) {
		return false
	}
	if !matchMetaRulesAll(meta, when.MetaAllNotIn, true) {
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

func matchMetaRulesAll(meta map[string]string, rules []string, negative bool) bool {
	if len(rules) == 0 {
		return true
	}

	matchedAll := true
	for _, rule := range rules {
		parts := strings.SplitN(rule, "=", 2)
		if len(parts) != 2 {
			matchedAll = false
			break
		}
		value := meta[parts[0]]
		ok, err := path.Match(parts[1], value)
		if err != nil || !ok {
			matchedAll = false
			break
		}
	}
	if negative {
		return !matchedAll
	}
	return matchedAll
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

func providerDebugLogPath(global ProvidersConfig, raw map[string]interface{}) string {
	value := providerString(raw, "debug_log")
	if value == "" {
		value = global.DebugLog
	}
	if value == "" {
		return ""
	}
	return providerExpandPath(value)
}

func writeProviderDebugLog(debugLogPath, name, method, executablePath string, input, stdout, stderr []byte, runErr error) {
	if debugLogPath == "" {
		return
	}

	if err := os.MkdirAll(filepath.Dir(debugLogPath), 0o755); err != nil {
		log.Printf("provider %q debug log mkdir failed: %v", name, err)
		return
	}

	file, err := os.OpenFile(debugLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		log.Printf("provider %q debug log open failed: %v", name, err)
		return
	}
	defer file.Close()

	sensitiveValues := collectProviderSensitiveValues(method, input, stdout)
	sanitizedInput := sanitizeProviderDebugJSON(method, input, true)
	sanitizedStdout := sanitizeProviderDebugJSON(method, stdout, false)
	sanitizedInput = sanitizeProviderDebugBytes(sanitizedInput, sensitiveValues)
	sanitizedStdout = sanitizeProviderDebugBytes(sanitizedStdout, sensitiveValues)
	sanitizedStderr := sanitizeProviderDebugBytes(stderr, sensitiveValues)
	sanitizedRunErr := sanitizeProviderDebugString(fmt.Sprint(runErr), sensitiveValues)

	_, _ = fmt.Fprintf(file, "[%s] provider=%s method=%s executable=%s\n", time.Now().Format(time.RFC3339), name, method, executablePath)
	_, _ = fmt.Fprintf(file, "request=%s\n", bytes.TrimSpace(sanitizedInput))
	if len(sanitizedStdout) > 0 {
		_, _ = fmt.Fprintf(file, "stdout=%s\n", bytes.TrimSpace(sanitizedStdout))
	}
	if len(sanitizedStderr) > 0 {
		_, _ = fmt.Fprintf(file, "stderr=%s\n", bytes.TrimSpace(sanitizedStderr))
	}
	if runErr != nil {
		_, _ = fmt.Fprintf(file, "error=%s\n", sanitizedRunErr)
	}
	_, _ = fmt.Fprintln(file)
}

func sanitizeProviderDebugJSON(method string, data []byte, isRequest bool) []byte {
	if len(bytes.TrimSpace(data)) == 0 {
		return data
	}

	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return data
	}

	sanitized := sanitizeProviderDebugValue(method, payload, nil, isRequest)
	marshaled, err := json.Marshal(sanitized)
	if err != nil {
		return data
	}

	return marshaled
}

func sanitizeProviderDebugValue(method string, value interface{}, path []string, isRequest bool) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		sanitized := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			nextPath := append(path, key)
			if shouldRedactProviderDebugField(method, nextPath, isRequest) {
				sanitized[key] = providerDebugRedactedValue(item)
				continue
			}
			sanitized[key] = sanitizeProviderDebugValue(method, item, nextPath, isRequest)
		}
		return sanitized
	case []interface{}:
		sanitized := make([]interface{}, len(typed))
		for i, item := range typed {
			sanitized[i] = sanitizeProviderDebugValue(method, item, path, isRequest)
		}
		return sanitized
	default:
		return value
	}
}

func providerDebugRedactedValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case []interface{}:
		redacted := make([]interface{}, len(typed))
		for i := range typed {
			redacted[i] = "<redacted>"
		}
		return redacted
	case map[string]interface{}:
		redacted := make(map[string]interface{}, len(typed))
		for key := range typed {
			redacted[key] = "<redacted>"
		}
		return redacted
	default:
		return "<redacted>"
	}
}

func collectProviderSensitiveValues(method string, input, stdout []byte) []string {
	values := map[string]struct{}{}
	collectProviderSensitiveValuesFromJSON(method, input, true, values)
	collectProviderSensitiveValuesFromJSON(method, stdout, false, values)

	result := make([]string, 0, len(values))
	for value := range values {
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		if len(result[i]) != len(result[j]) {
			return len(result[i]) > len(result[j])
		}
		return result[i] < result[j]
	})
	return result
}

func collectProviderSensitiveValuesFromJSON(method string, data []byte, isRequest bool, values map[string]struct{}) {
	if len(bytes.TrimSpace(data)) == 0 {
		return
	}

	var payload interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return
	}

	collectProviderSensitiveValue(method, payload, nil, isRequest, values)
}

func collectProviderSensitiveValue(method string, value interface{}, path []string, isRequest bool, values map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, item := range typed {
			nextPath := append(path, key)
			if shouldRedactProviderDebugField(method, nextPath, isRequest) {
				collectProviderSensitiveLeafStrings(item, values)
				continue
			}
			collectProviderSensitiveValue(method, item, nextPath, isRequest, values)
		}
	case []interface{}:
		for _, item := range typed {
			collectProviderSensitiveValue(method, item, path, isRequest, values)
		}
	}
}

func collectProviderSensitiveLeafStrings(value interface{}, values map[string]struct{}) {
	switch typed := value.(type) {
	case string:
		values[typed] = struct{}{}
	case []interface{}:
		for _, item := range typed {
			collectProviderSensitiveLeafStrings(item, values)
		}
	case map[string]interface{}:
		for _, item := range typed {
			collectProviderSensitiveLeafStrings(item, values)
		}
	}
}

func sanitizeProviderDebugBytes(data []byte, sensitiveValues []string) []byte {
	if len(data) == 0 {
		return data
	}
	sanitized := append([]byte(nil), data...)
	for _, value := range sensitiveValues {
		if value == "" {
			continue
		}
		sanitized = bytes.ReplaceAll(sanitized, []byte(value), []byte("<redacted>"))
	}
	return sanitized
}

func sanitizeProviderDebugString(value string, sensitiveValues []string) string {
	if value == "" {
		return value
	}
	sanitized := value
	for _, secret := range sensitiveValues {
		if secret == "" {
			continue
		}
		sanitized = strings.ReplaceAll(sanitized, secret, "<redacted>")
	}
	return sanitized
}

func shouldRedactProviderDebugField(method string, path []string, isRequest bool) bool {
	if len(path) == 0 {
		return false
	}

	last := path[len(path)-1]
	switch last {
	case "token", "token_secret", "password", "pass", "passes", "passphrase", "secret",
		"client_secret", "access_key", "secret_access_key", "session_token",
		"keypass", "keycmdpass", "certkeypass", "pin":
		return true
	}

	if method == providerapi.MethodSecretGet && !isRequest && len(path) >= 2 && path[0] == "result" && last == "value" {
		return true
	}

	return false
}

func providerExpandPath(value string) string {
	if value == "" {
		return ""
	}
	if value == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			value = home
		}
	} else if strings.HasPrefix(value, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
		}
	}
	if abs, err := filepath.Abs(value); err == nil {
		return abs
	}
	return value
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

func cloneProviderMeta(meta map[string]string) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(meta))
	for key, value := range meta {
		cloned[key] = value
	}
	return cloned
}

func mergeProviderConfigMaps(base, override map[string]interface{}) map[string]interface{} {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}

	result := make(map[string]interface{}, len(base)+len(override))
	for key, value := range base {
		result[key] = value
	}
	for key, value := range override {
		result[key] = value
	}
	return result
}

func parseSecretRef(ref string) (string, string, error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid provider secret ref %q", ref)
	}
	return parts[0], parts[1], nil
}

func resolveProviderExecutable(global ProvidersConfig, plugin string) (string, error) {
	candidates := providerExecutableCandidates(plugin)
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
