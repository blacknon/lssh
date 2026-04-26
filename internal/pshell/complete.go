// Copyright (c) 2024 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package pshell

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/c-bata/go-prompt"
	"golang.org/x/crypto/ssh"
)

func safeCreateSession(c *sConnect) (session *ssh.Session, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			session = nil
			err = fmt.Errorf("create session panic: %v", rec)
		}
	}()

	if c == nil || c.Connect == nil || c.Connect.Client == nil {
		return nil, fmt.Errorf("invalid connect")
	}

	return c.CreateSession()
}

func runCompleteCommand(c *sConnect, command string) (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	if c == nil || c.Connect == nil {
		return buf, fmt.Errorf("invalid connect")
	}

	if c.Connect.IsControlClient() {
		prevStdin := c.Connect.Stdin
		prevStdout := c.Connect.Stdout
		prevStderr := c.Connect.Stderr
		defer func() {
			c.Connect.Stdin = prevStdin
			c.Connect.Stdout = prevStdout
			c.Connect.Stderr = prevStderr
		}()

		c.Connect.Stdin = strings.NewReader("")
		c.Connect.Stdout = buf
		c.Connect.Stderr = io.Discard
		if err := c.Connect.Command(command); err != nil {
			return buf, err
		}

		return buf, nil
	}

	session, err := safeCreateSession(c)
	if err != nil || session == nil {
		return buf, err
	}
	defer session.Close()

	session.Stdout = buf
	if err := session.Run(command); err != nil {
		return buf, err
	}

	return buf, nil
}

var runCompleteCommandFn = runCompleteCommand

// TODO(blacknon): `!!`や"`:$`についても実装を行う
// TODO(blacknon): `!command`だとまとめてパイプ経由でデータを渡すことになっているが、`!!command`で個別のローカルコマンドにデータを渡すように実装する

// Completer parallel-shell complete function
func (s *shell) Completer(t prompt.Document) []prompt.Suggest {
	// if currente line data is none.
	if len(t.CurrentLine()) == 0 {
		return prompt.FilterHasPrefix(nil, t.GetWordBeforeCursor(), false)
	}

	left := t.CurrentLineBeforeCursor()
	wordBeforeCursor := t.GetWordBeforeCursor()
	targetConns := s.Connects
	targets := []string{}
	targeted := false

	targets, commandLeft, targetToken, inTargetSelector := parseLeadingTargetSelector(left, knownHostsFromConnects(s.Connects))
	if targetToken != "" {
		targeted = true
		if inTargetSelector {
			srvKey := targetToken
			if contains([]string{"@", ","}, lastChar(left)) || len(s.TargetSrvComp) == 0 || s.TargetSrvKey != srvKey {
				s.TargetSrvComp = s.buildTargetServerComplete(targetToken)
				s.TargetSrvKey = srvKey
			}
			return prompt.FilterHasPrefix(s.TargetSrvComp, targetToken, false)
		}

		left = commandLeft
		wordBeforeCursor = t.GetWordBeforeCursor()
		targetConns = s.filterTargetConnects(targets)

		cmdKey := strings.Join(targets, ",")
		if contains([]string{":", "@", ","}, lastChar(t.CurrentLineBeforeCursor())) || len(s.TargetCmdComp) == 0 || s.TargetCmdKey != cmdKey {
			s.TargetCmdComp = s.filterCommandComplete(targets)
			if len(s.TargetCmdComp) == 0 {
				s.TargetCmdComp = s.CmdComplete
			}
			s.TargetCmdKey = cmdKey
		}

		// Complete the first command token after `@server:` directly.
		// This keeps the behavior close to lsftp's host/path split:
		// generate on delimiters, then keep filtering as letters are typed.
		if !strings.ContainsAny(left, " |") {
			return prompt.FilterHasPrefix(s.TargetCmdComp, getCommandWord(left), false)
		}
	}

	pslice, err := parsePipeLine(left)
	if err != nil {
		return prompt.FilterHasPrefix(nil, wordBeforeCursor, false)
	}

	// Get cursor char(string)
	char := ""
	if len(left) > 0 {
		char = string(left[len(left)-1])
	}

	sl := len(pslice) // pline slice count
	ll := 0
	num := 0
	if sl >= 1 {
		ll = len(pslice[sl-1])             // pline count
		num = len(pslice[sl-1][ll-1].Args) // pline args count
	}

	if sl >= 1 && ll >= 1 {
		c := stripTargetPrefix(pslice[sl-1][ll-1].Args[0], knownHostsFromConnects(s.Connects))

		// switch suggest
		switch {
		case num <= 1 && !contains([]string{" ", "|"}, char): // if command
			var c []prompt.Suggest

			// build-in command suggest
			buildin := []prompt.Suggest{
				{Text: "exit", Description: "exit lssh shell"},
				{Text: "quit", Description: "exit lssh shell"},
				{Text: "clear", Description: "clear screen"},
				{Text: "%history", Description: "show history"},
				{Text: "%out", Description: "%out [num], show history result."},
				{Text: "%outlist", Description: "%outlist, show history result list."},
				{Text: "%outexec", Description: "%outexec <-n num> command..., exec local command with output result. result is in env variable."},
				{Text: "%get", Description: "%get remote local, copy files from remote hosts to localhost."},
				{Text: "%put", Description: "%put local... remote, copy local files to remote hosts."},
				{Text: "%reconnect", Description: "%reconnect [host...], reconnect disconnected hosts."},
				{Text: "%status", Description: "%status, show current connection status."},
				{Text: "%sync", Description: "%sync [--delete] [--dry-run] [-p] [-P num] (local|remote):source... (local|remote):target"},
				{Text: "%diff", Description: "%diff remote_path | @host:/path..., compare remote files in a synchronized TUI."},
				{Text: "%save", Description: "reserved built-in command."},
				{Text: "%set", Description: "reserved built-in command."},
				// outの出力でdiffをするためのローカルコマンド。すべての出力と比較するのはあまりに辛いと思われるため、最初の出力との比較、といった方式で対応するのが良いか？？
				// {Text: "%diff", Description: "%diff [num], show history result list."},
			}
			c = append(c, buildin...)

			// get remote and local command complete data
			filtered := s.CmdComplete
			if targeted {
				filtered = s.TargetCmdComp
			}
			c = append(c, filtered...)
			c = append(c, s.aliasSuggests()...)

			// return
			return prompt.FilterHasPrefix(c, wordBeforeCursor, false)

		case checkBuildInCommand(c): // if build-in command.
			suggest := s.getBuildInCommandSuggest(c, t, targetConns, num, char)
			return prompt.FilterHasPrefix(suggest, t.GetWordBeforeCursor(), false)

		default:
			switch {
			case strings.Contains(wordBeforeCursor, "/"):
				s.PathComplete = s.GetPathCompleteForConnects(targetConns, !checkLocalCommand(c), wordBeforeCursor)
			case strings.Count(t.CurrentLineBeforeCursor(), " ") >= 1:
				s.PathComplete = s.GetPathCompleteForConnects(targetConns, !checkLocalCommand(c), wordBeforeCursor)
			case contains([]string{" ", ":"}, char) && strings.Count(t.CurrentLineBeforeCursor(), " ") == 1:
				s.PathComplete = s.GetPathCompleteForConnects(targetConns, !checkLocalCommand(c), wordBeforeCursor)
			}

			return prompt.FilterHasPrefix(s.PathComplete, pathCompletionFilterWord(wordBeforeCursor), false)
		}
	}

	return prompt.FilterHasPrefix(nil, wordBeforeCursor, false)
}

func (s *shell) getBuildInCommandSuggest(command string, t prompt.Document, targetConns []*sConnect, num int, char string) []prompt.Suggest {
	switch command {
	case "%out":
		return s.getHistorySuggest()

	case "%outexec":
		switch {
		case contains([]string{"-"}, char):
			return []prompt.Suggest{
				{Text: "--help", Description: "help message"},
				{Text: "-h", Description: "help message"},
				{Text: "-n", Description: "set history number"},
			}
		case "-n " == t.GetWordBeforeCursorWithSpace():
			return s.getHistorySuggest()
		default:
			return s.GetLocalhostCommandComplete()
		}

	case "%get":
		switch {
		case contains([]string{"-"}, char):
			return []prompt.Suggest{
				{Text: "--dry-run", Description: "show get actions without modifying files"},
			}
		case (num == 1 && char == " ") || (num == 2 && char != " "):
			return s.GetPathCompleteForConnects(targetConns, true, t.GetWordBeforeCursor())
		case (num == 2 && char == " ") || num >= 3:
			return s.GetPathComplete(false, t.GetWordBeforeCursor())
		}

	case "%put":
		switch {
		case contains([]string{"-"}, char):
			return []prompt.Suggest{
				{Text: "--dry-run", Description: "show put actions without modifying files"},
			}
		case num == 1 || (num == 2 && char != " "):
			return s.GetPathComplete(false, t.GetWordBeforeCursor())
		case num >= 2 && char == " ":
			return appendPathSuggests(
				s.GetPathComplete(false, t.GetWordBeforeCursor()),
				s.GetPathCompleteForConnects(targetConns, true, t.GetWordBeforeCursor()),
			)
		case num >= 3:
			return s.GetPathComplete(false, t.GetWordBeforeCursor())
		}

	case "%sync":
		switch {
		case contains([]string{"-"}, char):
			return []prompt.Suggest{
				{Text: "--delete", Description: "delete destination entries not present in source"},
				{Text: "--dry-run", Description: "show sync actions without modifying files"},
				{Text: "--permission", Description: "copy file permission"},
				{Text: "-p", Description: "copy file permission"},
				{Text: "--parallel", Description: "parallel file sync count per host"},
				{Text: "-P", Description: "parallel file sync count per host"},
			}
		default:
			return appendPathSuggests(
				[]prompt.Suggest{
					{Text: "local:", Description: "local path"},
					{Text: "remote:", Description: "remote path"},
					{Text: "remote:@", Description: "remote path with host selector"},
				},
				s.GetPathComplete(false, t.GetWordBeforeCursor()),
				s.GetPathCompleteForConnects(targetConns, true, t.GetWordBeforeCursor()),
			)
		}

	case "%diff":
		return appendPathSuggests(
			[]prompt.Suggest{
				{Text: "@", Description: "explicit remote target (@host:/path)"},
			},
			s.GetPathCompleteForConnects(targetConns, true, t.GetWordBeforeCursor()),
		)

	case "%reconnect":
		return s.getServerStatusSuggests()

	case "%status":
		return nil
	}

	return nil
}

func (s *shell) getServerStatusSuggests() []prompt.Suggest {
	result := make([]prompt.Suggest, 0, len(s.Connects))
	for _, conn := range s.Connects {
		if conn == nil {
			continue
		}
		desc := "connected"
		if !conn.Connected {
			desc = "disconnected"
		}
		result = append(result, prompt.Suggest{Text: conn.Name, Description: desc})
	}
	return result
}

func (s *shell) getHistorySuggest() []prompt.Suggest {
	suggest := make([]prompt.Suggest, 0, len(s.History))
	for i := 0; i < len(s.History); i++ {
		var cmd string
		for _, h := range s.History[i] {
			cmd = h.Command
		}

		suggest = append(suggest, prompt.Suggest{
			Text:        strconv.Itoa(i),
			Description: cmd,
		})
	}

	return suggest
}

func appendPathSuggests(groups ...[]prompt.Suggest) []prompt.Suggest {
	result := make([]prompt.Suggest, 0)
	seen := map[string]struct{}{}

	for _, group := range groups {
		for _, suggest := range group {
			key := suggest.Text + "\x00" + suggest.Description
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, suggest)
		}
	}

	sort.SliceStable(result, func(i, j int) bool { return result[i].Text < result[j].Text })
	return result
}

func filterConnectedConnects(connects []*sConnect) []*sConnect {
	filtered := make([]*sConnect, 0, len(connects))
	for _, conn := range connects {
		if conn == nil || conn.Connect == nil || !conn.Connected {
			continue
		}
		filtered = append(filtered, conn)
	}

	return filtered
}

func filterLiveRemoteCompletionConnects(connects []*sConnect) []*sConnect {
	return filterConnectedConnects(connects)
}

// GetLocalhostCommandComplete
func (s *shell) GetLocalhostCommandComplete() (suggest []prompt.Suggest) {
	return localCommandSuggests()
}

// GetCommandComplete get command list remote machine.
// mode ... command/path
// data ... Value being entered
func (s *shell) GetCommandComplete() {
	command := "compgen -c"
	for _, suggest := range localCommandSuggests() {
		s.CmdComplete = append(s.CmdComplete, prompt.Suggest{
			Text:        "+" + suggest.Text,
			Description: suggest.Description,
		})
	}

	// get remote machine command complete
	// command map
	cmdMap := map[string][]string{}

	// append command to cmdMap
	for _, c := range filterConnectedConnects(s.Connects) {
		if c == nil || c.Connect == nil {
			continue
		}

		buf, err := runCompleteCommandFn(c, command)
		if err != nil {
			continue
		}

		// Scan and put completed command to map.
		sc := bufio.NewScanner(buf)
		for sc.Scan() {
			cmdMap[sc.Text()] = append(cmdMap[sc.Text()], c.Name)
		}
	}

	// cmdMap to suggest
	for cmd, hosts := range cmdMap {
		// join hosts
		sort.Strings(hosts)
		h := strings.Join(hosts, ",")

		// create suggest
		suggest := prompt.Suggest{
			Text:        cmd,
			Description: "Command. from:" + h,
		}

		// append s.Complete
		s.CmdComplete = append(s.CmdComplete, suggest)
	}

	s.CmdComplete = append(s.CmdComplete, s.aliasSuggests()...)

	sort.SliceStable(s.CmdComplete, func(i, j int) bool { return s.CmdComplete[i].Text < s.CmdComplete[j].Text })
}

// GetPathComplete return complete path from local or remote machine.
// TODO(blacknon): 複数のノードにあるPATHだけ補完リストに出てる状態なので、単一ノードにしか無いファイルも出力されるよう修正する
func (s *shell) GetPathComplete(remote bool, word string) (p []prompt.Suggest) {
	return s.GetPathCompleteForConnects(filterConnectedConnects(s.Connects), remote, word)
}

func (s *shell) GetPathCompleteForConnects(connects []*sConnect, remote bool, word string) (p []prompt.Suggest) {
	command := remotePathCompleteCommand(word)

	switch {
	case remote: // is remote machine
		connects = filterLiveRemoteCompletionConnects(connects)
		if len(connects) == 0 {
			return nil
		}

		// create map
		m := map[string][]string{}

		exit := make(chan bool)

		// create sync mutex
		sm := new(sync.Mutex)

		// append path to m
		for _, c := range connects {
			con := c
			go func() {
				if con == nil || con.Connect == nil || !con.Connected {
					exit <- true
					return
				}

				buf, err := runCompleteCommandFn(con, command)
				if err != nil {
					exit <- true
					return
				}

				// Scan and put completed command to map.
				sc := bufio.NewScanner(buf)
				for sc.Scan() {
					sm.Lock()

					path := sc.Text()

					m[path] = append(m[path], con.Name)
					sm.Unlock()
				}

				exit <- true
			}()
		}

		for i := 0; i < len(connects); i++ {
			<-exit
		}

		// m to suggest
		for path, hosts := range m {
			// join hosts
			h := strings.Join(hosts, ",")

			// create suggest
			suggest := prompt.Suggest{
				Text:        pathCompletionText(path),
				Description: "remote path. from:" + h,
			}

			// append s.Complete
			p = append(p, suggest)
		}

	case !remote: // is local machine
		p = append(p, localPathSuggests(word)...)
	}

	sort.SliceStable(p, func(i, j int) bool { return p[i].Text < p[j].Text })
	return
}

func localCommandSuggests() []prompt.Suggest {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil
	}

	seen := map[string]struct{}{}
	suggests := make([]prompt.Suggest, 0)
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			name := entry.Name()
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			if !isExecutableEntry(dir, entry) {
				continue
			}

			seen[name] = struct{}{}
			suggests = append(suggests, prompt.Suggest{
				Text:        name,
				Description: "Command. from:localhost",
			})
		}
	}

	sort.SliceStable(suggests, func(i, j int) bool { return suggests[i].Text < suggests[j].Text })
	return suggests
}

func localPathSuggests(word string) []prompt.Suggest {
	search := word
	if search == "" {
		search = "."
	}

	dir, prefix := filepath.Split(search)
	if dir == "" {
		dir = "."
	}

	textPrefix := dir
	if textPrefix == "." {
		textPrefix = ""
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	suggests := make([]prompt.Suggest, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}

		suggests = append(suggests, prompt.Suggest{
			Text:        name,
			Description: "local path. " + textPrefix + name,
		})
	}

	sort.SliceStable(suggests, func(i, j int) bool { return suggests[i].Text < suggests[j].Text })
	return suggests
}

func remotePathCompleteCommand(word string) string {
	quoted := shellQuote(word)
	return "for p in $(compgen -f -- " + quoted + "); do if [ -d \"$p\" ]; then p=${p%/}; printf '%s/\\n' \"$p\"; else printf '%s\\n' \"$p\"; fi; done"
}

func pathCompletionFilterWord(word string) string {
	idx := strings.LastIndexAny(word, `/\`)
	if idx >= 0 {
		return word[idx+1:]
	}

	return word
}

func pathCompletionText(candidate string) string {
	if candidate == "" {
		return ""
	}

	trimmed := strings.Trim(candidate, `/\`)
	if trimmed == "" {
		return ""
	}

	idx := strings.LastIndexAny(trimmed, `/\`)
	if idx >= 0 {
		return trimmed[idx+1:]
	}

	return trimmed
}

func isExecutableEntry(dir string, entry os.DirEntry) bool {
	if entry.IsDir() {
		return false
	}

	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		for _, candidate := range executableExtensions() {
			if ext == candidate {
				return true
			}
		}
		return false
	}

	info, err := entry.Info()
	if err != nil {
		return false
	}
	mode := info.Mode()
	if mode.IsRegular() && mode&0o111 != 0 {
		return true
	}

	fullPath := filepath.Join(dir, entry.Name())
	info, err = os.Stat(fullPath)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Mode()&0o111 != 0
}

func executableExtensions() []string {
	pathExt := os.Getenv("PATHEXT")
	if pathExt == "" {
		return []string{".com", ".exe", ".bat", ".cmd"}
	}

	parts := filepath.SplitList(pathExt)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		result = append(result, strings.ToLower(part))
	}
	return result
}

func parseLeadingTargetSelector(line string, knownHosts []string) (targets []string, command string, token string, inSelector bool) {
	line = strings.TrimLeft(line, " ")
	if line == "" || line[0] != '@' {
		return nil, "", "", false
	}

	token = line
	if idx := strings.IndexAny(token, " |"); idx >= 0 {
		token = token[:idx]
	}

	if !strings.HasPrefix(token, "@") {
		return nil, "", "", false
	}

	if parsedTargets, commandHead, err := parseTargetedCommand(token, knownHosts); err == nil {
		remaining := strings.TrimPrefix(line, token)
		return parsedTargets, commandHead + remaining, token, false
	}

	value := strings.TrimPrefix(token, "@")
	hosts := strings.Split(value, ",")
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if host != "" {
			targets = append(targets, host)
		}
	}

	return targets, "", token, true
}

func stripTargetPrefix(command string, knownHosts []string) string {
	if !strings.HasPrefix(command, "@") {
		return command
	}

	if _, cmd, _, inSelector := parseLeadingTargetSelector(command, knownHosts); !inSelector && cmd != "" {
		return cmd
	}

	return command
}

func getCommandWord(line string) string {
	line = strings.TrimLeft(line, " ")
	if line == "" {
		return ""
	}

	if idx := strings.LastIndexAny(line, " |"); idx >= 0 {
		return line[idx+1:]
	}

	return line
}

func (s *shell) buildTargetServerComplete(token string) []prompt.Suggest {
	hostsPart := strings.TrimPrefix(token, "@")
	base := "@"
	selected := map[string]struct{}{}

	if idx := strings.LastIndex(hostsPart, ","); idx >= 0 {
		base += hostsPart[:idx+1]
		for _, host := range strings.Split(hostsPart[:idx], ",") {
			host = strings.TrimSpace(host)
			if host != "" {
				selected[host] = struct{}{}
			}
		}
	}

	servers := make([]string, 0, len(s.Connects))
	for _, con := range s.Connects {
		if con == nil {
			continue
		}
		if _, ok := selected[con.Name]; ok {
			continue
		}
		servers = append(servers, con.Name)
	}
	sort.Strings(servers)

	suggest := make([]prompt.Suggest, 0, len(servers)*2)
	for _, server := range servers {
		text := base + server
		suggest = append(suggest, prompt.Suggest{
			Text:        text + ":",
			Description: "target server.",
		})
		suggest = append(suggest, prompt.Suggest{
			Text:        text + ",",
			Description: "add target server.",
		})
	}

	return suggest
}

func (s *shell) filterTargetConnects(targets []string) []*sConnect {
	if len(targets) == 0 {
		return s.Connects
	}

	targetMap := map[string]struct{}{}
	for _, target := range targets {
		targetMap[target] = struct{}{}
	}

	connects := make([]*sConnect, 0, len(targets))
	for _, con := range s.Connects {
		if con == nil {
			continue
		}
		if _, ok := targetMap[con.Name]; ok {
			connects = append(connects, con)
		}
	}

	if len(connects) == 0 {
		return s.Connects
	}

	return connects
}

func (s *shell) filterCommandComplete(targets []string) []prompt.Suggest {
	if len(targets) == 0 {
		return s.CmdComplete
	}

	targetMap := map[string]struct{}{}
	for _, target := range targets {
		targetMap[target] = struct{}{}
	}

	filtered := make([]prompt.Suggest, 0, len(s.CmdComplete))
	for _, suggest := range s.CmdComplete {
		if strings.HasPrefix(suggest.Text, "+") {
			filtered = append(filtered, suggest)
			continue
		}

		hosts := strings.TrimPrefix(suggest.Description, "Command. from:")
		if hosts == suggest.Description {
			filtered = append(filtered, suggest)
			continue
		}

		hostMap := map[string]struct{}{}
		for _, host := range strings.Split(hosts, ",") {
			hostMap[strings.TrimSpace(host)] = struct{}{}
		}

		match := true
		for target := range targetMap {
			if _, ok := hostMap[target]; !ok {
				match = false
				break
			}
		}

		if match {
			filtered = append(filtered, suggest)
		}
	}

	return filtered
}

func lastChar(s string) string {
	if s == "" {
		return ""
	}

	return string(s[len(s)-1])
}

func contains(s []string, e string) bool {
	for _, v := range s {
		if e == v {
			return true
		}
	}
	return false
}
