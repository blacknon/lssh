package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/blacknon/lssh/providerapi"
	"github.com/kballard/go-shellquote"
	"gopkg.in/yaml.v3"
)

const (
	ansiblePluginName = "provider-inventory-ansible"
)

func main() {
	req, err := providerapi.ReadRequest()
	if err != nil {
		_ = providerapi.WriteError(err.Error())
		os.Exit(1)
	}

	switch req.Method {
	case providerapi.MethodPluginDescribe:
		_ = providerapi.WriteResponse(req, providerapi.PluginDescribeResult{
			Name:         ansiblePluginName,
			Capabilities: []string{"inventory"},
			Methods:      []string{providerapi.MethodPluginDescribe, providerapi.MethodHealthCheck, providerapi.MethodInventoryList},
			ReservedKeys: []string{
				"inventory_file", "inventory_format",
				"include_groups", "exclude_groups",
				"server_name_template", "addr_template", "note_template",
			},
			ProtocolVersion: providerapi.Version,
		}, nil)
	case providerapi.MethodInventoryList:
		var params providerapi.InventoryListParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}

		servers, err := listAnsibleInventory(params.Config)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "inventory_failed", err.Error())
			os.Exit(1)
		}
		_ = providerapi.WriteResponse(req, providerapi.InventoryListResult{Servers: servers}, nil)
	case providerapi.MethodHealthCheck:
		var params providerapi.HealthCheckParams
		if err := decodeParams(req.Params, &params); err != nil {
			_ = providerapi.WriteErrorResponse(req, "invalid_params", err.Error())
			os.Exit(1)
		}
		result, err := ansibleHealthCheck(params.Config)
		if err != nil {
			_ = providerapi.WriteErrorResponse(req, "health_check_failed", err.Error())
			os.Exit(1)
		}
		_ = providerapi.WriteResponse(req, result, nil)
	default:
		_ = providerapi.WriteErrorResponse(req, "unsupported_method", fmt.Sprintf("unsupported method %q", req.Method))
		os.Exit(1)
	}
}

func decodeParams(raw interface{}, out interface{}) error {
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

type ansibleInventory struct {
	Groups map[string]*ansibleGroup
}

type ansibleGroup struct {
	Name     string
	Vars     map[string]string
	Hosts    map[string]map[string]string
	Children []string
}

type ansibleHostEntry struct {
	Name   string
	Vars   map[string]string
	Groups []string
}

func listAnsibleInventory(config map[string]interface{}) ([]providerapi.InventoryServer, error) {
	hosts, inventoryPath, err := loadAnsibleInventory(config)
	if err != nil {
		return nil, err
	}

	includeGroups := stringSet(providerapi.StringSlice(config, "include_groups"))
	excludeGroups := stringSet(providerapi.StringSlice(config, "exclude_groups"))
	nameTemplate := providerapi.String(config, "server_name_template")
	if nameTemplate == "" {
		nameTemplate = "ansible:${inventory_hostname}"
	}
	addrTemplate := providerapi.String(config, "addr_template")
	noteTemplate := providerapi.String(config, "note_template")

	out := make([]providerapi.InventoryServer, 0, len(hosts))
	for _, host := range hosts {
		if !ansibleHostIncluded(host.Groups, includeGroups, excludeGroups) {
			continue
		}

		values := ansibleTemplateValues(host, inventoryPath)
		addr := host.Vars["ansible_host"]
		if addr == "" {
			addr = host.Name
		}
		if addrTemplate != "" {
			addr = renderTemplate(addrTemplate, values)
		}

		user := firstNonEmpty(host.Vars["ansible_user"], host.Vars["ansible_ssh_user"])
		password := firstNonEmpty(host.Vars["ansible_password"], host.Vars["ansible_ssh_pass"])
		port := host.Vars["ansible_port"]
		key := host.Vars["ansible_ssh_private_key_file"]

		note := noteTemplate
		if note == "" {
			if values["groups"] == "" {
				note = "ansible"
			} else {
				note = "ansible " + values["groups"]
			}
		}
		note = strings.TrimSpace(renderTemplate(note, values))

		meta := map[string]string{
			"provider":           "ansible",
			"plugin":             ansiblePluginName,
			"inventory_hostname": host.Name,
			"name":               host.Name,
			"addr":               addr,
			"source_path":        inventoryPath,
			"groups":             values["groups"],
		}
		if user != "" {
			meta["ansible_user"] = user
		}
		if addr != "" {
			meta["ansible_host"] = addr
		}
		if port != "" {
			meta["ansible_port"] = port
		}
		for _, group := range host.Groups {
			meta["group."+group] = "true"
		}
		for key, value := range host.Vars {
			if value == "" {
				continue
			}
			meta["var."+key] = value
		}

		serverConfig := map[string]interface{}{
			"addr": addr,
			"note": note,
		}
		if user != "" {
			serverConfig["user"] = user
		}
		if password != "" {
			serverConfig["pass"] = password
		}
		if port != "" {
			serverConfig["port"] = port
		}
		if key != "" {
			serverConfig["key"] = key
		}

		out = append(out, providerapi.InventoryServer{
			Name:   renderTemplate(nameTemplate, values),
			Config: serverConfig,
			Meta:   meta,
		})
	}

	return out, nil
}

func ansibleHealthCheck(config map[string]interface{}) (providerapi.HealthCheckResult, error) {
	hosts, inventoryPath, err := loadAnsibleInventory(config)
	if err != nil {
		return providerapi.HealthCheckResult{}, err
	}
	return providerapi.HealthCheckResult{
		OK:      true,
		Message: fmt.Sprintf("loaded %d hosts from %s", len(hosts), inventoryPath),
	}, nil
}

func loadAnsibleInventory(config map[string]interface{}) ([]ansibleHostEntry, string, error) {
	inventoryFile := strings.TrimSpace(providerapi.String(config, "inventory_file"))
	if inventoryFile == "" {
		return nil, "", fmt.Errorf("inventory_file is required")
	}

	inventoryPath := providerapi.ExpandPath(inventoryFile)
	data, err := os.ReadFile(inventoryPath)
	if err != nil {
		return nil, "", err
	}

	format, err := ansibleInventoryFormat(config, inventoryPath)
	if err != nil {
		return nil, "", err
	}

	var inventory ansibleInventory
	switch format {
	case "ini":
		inventory, err = parseAnsibleINIInventory(string(data))
	case "yaml":
		inventory, err = parseAnsibleYAMLInventory(data)
	case "json":
		inventory, err = parseAnsibleJSONInventory(data)
	default:
		err = fmt.Errorf("unsupported inventory_format %q", format)
	}
	if err != nil {
		return nil, "", err
	}

	hosts := inventory.flattenHosts()
	sort.Slice(hosts, func(i, j int) bool {
		return hosts[i].Name < hosts[j].Name
	})

	return hosts, inventoryPath, nil
}

func ansibleInventoryFormat(config map[string]interface{}, inventoryPath string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(providerapi.String(config, "inventory_format")))
	if format == "" || format == "auto" {
		switch strings.ToLower(filepath.Ext(inventoryPath)) {
		case ".yaml", ".yml":
			return "yaml", nil
		case ".json":
			return "json", nil
		default:
			return "ini", nil
		}
	}

	switch format {
	case "ini", "yaml", "json":
		return format, nil
	default:
		return "", fmt.Errorf("inventory_format must be one of auto, ini, yaml, json")
	}
}

func (inv ansibleInventory) flattenHosts() []ansibleHostEntry {
	if len(inv.Groups) == 0 {
		return nil
	}

	entries := map[string]*ansibleHostEntry{}

	var walk func(name string, inheritedVars map[string]string, inheritedGroups []string, stack map[string]bool)
	walk = func(name string, inheritedVars map[string]string, inheritedGroups []string, stack map[string]bool) {
		stack = cloneBoolMap(stack)
		if stack[name] {
			return
		}
		stack[name] = true

		group := inv.ensureGroup(name)
		groupVars := mergeStringMaps(inheritedVars, group.Vars)
		groupTrail := appendUnique(append([]string{}, inheritedGroups...), name)

		hostNames := make([]string, 0, len(group.Hosts))
		for hostName := range group.Hosts {
			hostNames = append(hostNames, hostName)
		}
		sort.Strings(hostNames)
		for _, hostName := range hostNames {
			hostVars := mergeStringMaps(groupVars, group.Hosts[hostName])
			entry := entries[hostName]
			if entry == nil {
				entry = &ansibleHostEntry{Name: hostName, Vars: map[string]string{}, Groups: nil}
				entries[hostName] = entry
			}
			entry.Vars = mergeStringMaps(entry.Vars, hostVars)
			entry.Groups = appendUnique(entry.Groups, groupTrail...)
		}

		children := append([]string{}, group.Children...)
		sort.Strings(children)
		for _, child := range children {
			walk(child, groupVars, groupTrail, stack)
		}
	}

	allVars := map[string]string{}
	if allGroup, ok := inv.Groups["all"]; ok {
		allVars = cloneStringMap(allGroup.Vars)
		walk("all", nil, nil, nil)
	}

	groupNames := make([]string, 0, len(inv.Groups))
	for name := range inv.Groups {
		if name == "all" {
			continue
		}
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)
	for _, name := range groupNames {
		walk(name, allVars, nil, nil)
	}

	out := make([]ansibleHostEntry, 0, len(entries))
	for _, entry := range entries {
		entry.Groups = normalizedGroups(entry.Groups)
		out = append(out, *entry)
	}
	return out
}

func (inv ansibleInventory) ensureGroup(name string) *ansibleGroup {
	if inv.Groups == nil {
		inv.Groups = map[string]*ansibleGroup{}
	}
	group, ok := inv.Groups[name]
	if !ok {
		group = &ansibleGroup{Name: name, Vars: map[string]string{}, Hosts: map[string]map[string]string{}}
		inv.Groups[name] = group
	}
	if group.Vars == nil {
		group.Vars = map[string]string{}
	}
	if group.Hosts == nil {
		group.Hosts = map[string]map[string]string{}
	}
	return group
}

func parseAnsibleINIInventory(body string) (ansibleInventory, error) {
	inv := ansibleInventory{Groups: map[string]*ansibleGroup{}}
	currentGroup := "ungrouped"
	currentMode := "hosts"

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.TrimSpace(line[1 : len(line)-1])
			currentMode = "hosts"
			currentGroup = section
			if strings.HasSuffix(section, ":vars") {
				currentMode = "vars"
				currentGroup = strings.TrimSuffix(section, ":vars")
			} else if strings.HasSuffix(section, ":children") {
				currentMode = "children"
				currentGroup = strings.TrimSuffix(section, ":children")
			}
			inv.ensureGroup(currentGroup)
			continue
		}

		group := inv.ensureGroup(currentGroup)
		switch currentMode {
		case "vars":
			key, value, ok := splitINIKeyValue(line)
			if ok {
				group.Vars[key] = value
			}
		case "children":
			child := strings.TrimSpace(line)
			if child == "" {
				continue
			}
			group.Children = appendUnique(group.Children, child)
			inv.ensureGroup(child)
		default:
			parts, err := shellquote.Split(line)
			if err != nil {
				return ansibleInventory{}, fmt.Errorf("invalid host line %q: %w", line, err)
			}
			if len(parts) == 0 {
				continue
			}

			hostVars := map[string]string{}
			for _, part := range parts[1:] {
				key, value, ok := splitINIKeyValue(part)
				if !ok {
					continue
				}
				hostVars[key] = value
			}

			for _, hostName := range expandAnsibleHostPattern(parts[0]) {
				group.Hosts[hostName] = mergeStringMaps(group.Hosts[hostName], hostVars)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return ansibleInventory{}, err
	}

	return inv, nil
}

func parseAnsibleYAMLInventory(data []byte) (ansibleInventory, error) {
	var raw interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ansibleInventory{}, err
	}
	return parseAnsibleStructuredInventory(raw)
}

func parseAnsibleJSONInventory(data []byte) (ansibleInventory, error) {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ansibleInventory{}, err
	}
	return parseAnsibleStructuredInventory(raw)
}

func parseAnsibleStructuredInventory(raw interface{}) (ansibleInventory, error) {
	root := toStringInterfaceMap(raw)
	if root == nil {
		return ansibleInventory{}, fmt.Errorf("inventory must decode to an object")
	}

	inv := ansibleInventory{Groups: map[string]*ansibleGroup{}}
	if looksLikeStructuredGroup(root) {
		parseStructuredGroup(inv.ensureGroup("all"), root, inv)
		return inv, nil
	}

	groupNames := make([]string, 0, len(root))
	for name := range root {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)
	for _, name := range groupNames {
		groupBody := toStringInterfaceMap(root[name])
		if groupBody == nil {
			groupBody = map[string]interface{}{}
		}
		parseStructuredGroup(inv.ensureGroup(name), groupBody, inv)
	}

	return inv, nil
}

func looksLikeStructuredGroup(raw map[string]interface{}) bool {
	_, hasHosts := raw["hosts"]
	_, hasVars := raw["vars"]
	_, hasChildren := raw["children"]
	return hasHosts || hasVars || hasChildren
}

func parseStructuredGroup(group *ansibleGroup, raw map[string]interface{}, inv ansibleInventory) {
	group.Vars = mergeStringMaps(group.Vars, scalarStringMap(raw["vars"]))

	for hostName, hostRaw := range toStringInterfaceMap(raw["hosts"]) {
		group.Hosts[hostName] = mergeStringMaps(group.Hosts[hostName], scalarStringMap(hostRaw))
	}

	for childName, childRaw := range toStringInterfaceMap(raw["children"]) {
		group.Children = appendUnique(group.Children, childName)
		childGroup := inv.ensureGroup(childName)
		childBody := toStringInterfaceMap(childRaw)
		if childBody == nil {
			childBody = map[string]interface{}{}
		}
		parseStructuredGroup(childGroup, childBody, inv)
	}
}

func ansibleHostIncluded(groups []string, includeGroups, excludeGroups map[string]bool) bool {
	for _, group := range groups {
		if excludeGroups[group] {
			return false
		}
	}
	if len(includeGroups) == 0 {
		return true
	}
	for _, group := range groups {
		if includeGroups[group] {
			return true
		}
	}
	return false
}

func ansibleTemplateValues(host ansibleHostEntry, inventoryPath string) map[string]string {
	values := map[string]string{
		"inventory_hostname": host.Name,
		"name":               host.Name,
		"inventory_file":     inventoryPath,
		"groups":             strings.Join(host.Groups, ","),
		"group_csv":          strings.Join(host.Groups, ","),
	}
	for key, value := range host.Vars {
		values[key] = value
	}
	if values["ansible_host"] == "" {
		values["ansible_host"] = host.Name
	}
	return values
}

var templatePattern = regexp.MustCompile(`\$\{([^}]+)\}`)

func renderTemplate(template string, values map[string]string) string {
	return templatePattern.ReplaceAllStringFunc(template, func(match string) string {
		submatches := templatePattern.FindStringSubmatch(match)
		if len(submatches) != 2 {
			return ""
		}
		return values[submatches[1]]
	})
}

func splitINIKeyValue(input string) (string, string, bool) {
	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" {
		return "", "", false
	}
	return key, strings.Trim(value, `"'`), true
}

var hostPatternRegexp = regexp.MustCompile(`^(.*)\[([^\[\]]+)\](.*)$`)

func expandAnsibleHostPattern(pattern string) []string {
	matches := hostPatternRegexp.FindStringSubmatch(pattern)
	if len(matches) != 4 {
		return []string{pattern}
	}

	prefix, spec, suffix := matches[1], matches[2], matches[3]
	parts := strings.Split(spec, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return []string{pattern}
	}
	step := 1
	if len(parts) == 3 {
		parsedStep, err := strconv.Atoi(parts[2])
		if err != nil || parsedStep <= 0 {
			return []string{pattern}
		}
		step = parsedStep
	}

	if expanded, ok := expandNumericHostPattern(prefix, suffix, parts[0], parts[1], step); ok {
		return expanded
	}
	if expanded, ok := expandAlphaHostPattern(prefix, suffix, parts[0], parts[1], step); ok {
		return expanded
	}

	return []string{pattern}
}

func expandNumericHostPattern(prefix, suffix, startText, endText string, step int) ([]string, bool) {
	start, errStart := strconv.Atoi(startText)
	end, errEnd := strconv.Atoi(endText)
	if errStart != nil || errEnd != nil {
		return nil, false
	}

	width := len(startText)
	if len(endText) > width {
		width = len(endText)
	}

	var out []string
	if start <= end {
		for i := start; i <= end; i += step {
			out = append(out, fmt.Sprintf("%s%0*d%s", prefix, width, i, suffix))
		}
		return out, true
	}

	for i := start; i >= end; i -= step {
		out = append(out, fmt.Sprintf("%s%0*d%s", prefix, width, i, suffix))
	}
	return out, true
}

func expandAlphaHostPattern(prefix, suffix, startText, endText string, step int) ([]string, bool) {
	if len(startText) != 1 || len(endText) != 1 {
		return nil, false
	}

	start := rune(startText[0])
	end := rune(endText[0])
	var out []string
	if start <= end {
		for r := start; r <= end; r += rune(step) {
			out = append(out, fmt.Sprintf("%s%c%s", prefix, r, suffix))
		}
		return out, true
	}

	for r := start; r >= end; r -= rune(step) {
		out = append(out, fmt.Sprintf("%s%c%s", prefix, r, suffix))
	}
	return out, true
}

func toStringInterfaceMap(raw interface{}) map[string]interface{} {
	switch value := raw.(type) {
	case map[string]interface{}:
		return value
	case map[interface{}]interface{}:
		result := make(map[string]interface{}, len(value))
		for key, item := range value {
			result[fmt.Sprint(key)] = item
		}
		return result
	default:
		return nil
	}
}

func scalarStringMap(raw interface{}) map[string]string {
	switch value := raw.(type) {
	case nil:
		return map[string]string{}
	case map[string]string:
		return cloneStringMap(value)
	case map[string]interface{}:
		result := make(map[string]string, len(value))
		for key, item := range value {
			if item == nil {
				continue
			}
			result[key] = fmt.Sprint(item)
		}
		return result
	case map[interface{}]interface{}:
		result := make(map[string]string, len(value))
		for key, item := range value {
			if item == nil {
				continue
			}
			result[fmt.Sprint(key)] = fmt.Sprint(item)
		}
		return result
	default:
		return map[string]string{}
	}
}

func stringSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result[trimmed] = true
		}
	}
	return result
}

func cloneBoolMap(input map[string]bool) map[string]bool {
	if len(input) == 0 {
		return map[string]bool{}
	}
	result := make(map[string]bool, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func mergeStringMaps(base, override map[string]string) map[string]string {
	result := cloneStringMap(base)
	for key, value := range override {
		result[key] = value
	}
	return result
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func appendUnique(values []string, extras ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range extras {
		if value == "" || seen[value] {
			continue
		}
		values = append(values, value)
		seen[value] = true
	}
	return values
}

func normalizedGroups(groups []string) []string {
	filtered := make([]string, 0, len(groups))
	for _, group := range groups {
		if group == "" || group == "all" {
			continue
		}
		filtered = appendUnique(filtered, group)
	}
	sort.Strings(filtered)
	return filtered
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
