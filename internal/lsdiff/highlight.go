// Copyright (c) 2026 Blacknon. All rights reserved.
// Use of this source code is governed by an MIT license
// that can be found in the LICENSE file.

package lsdiff

import (
	"regexp"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	"github.com/rivo/tview"
)

type textStyle struct {
	fg   string
	bg   string
	bold bool
}

var (
	syntaxStringPattern  = regexp.MustCompile(`"([^"\\]|\\.)*"|'([^'\\]|\\.)*'|` + "`" + `[^` + "`" + `]*` + "`")
	syntaxNumberPattern  = regexp.MustCompile(`\b[-+]?(0x[0-9a-fA-F]+|[0-9]+(\.[0-9]+)?)\b`)
	syntaxJSONKeyPattern = regexp.MustCompile(`"([^"\\]|\\.)*"\s*:`)
)

func renderStyledLine(path, line, reference, searchTerm string, changed bool) string {
	runes := []rune(line)
	styles := make([]textStyle, len(runes))
	for i := range styles {
		styles[i] = textStyle{fg: "white"}
	}

	applySyntaxHighlight(path, runes, styles)
	if changed {
		applyInlineDiffHighlight(reference, line, styles)
	}
	if searchTerm != "" {
		applySearchHighlight(line, searchTerm, styles)
	}

	return renderWithStyles(runes, styles)
}

func applySyntaxHighlight(path string, runes []rune, styles []textStyle) {
	line := string(runes)
	lang := detectSyntax(path)

	switch lang {
	case "go":
		highlightComment(line, styles, "//", "gray")
		highlightKeywords(line, styles, []string{"package", "import", "func", "type", "struct", "interface", "return", "if", "else", "for", "range", "switch", "case", "var", "const"}, "deepskyblue", true)
		highlightKeywords(line, styles, []string{"true", "false", "nil"}, "mediumpurple", true)
		highlightStrings(line, styles, "gold")
		highlightNumbers(line, styles, "lightsalmon")
	case "sh":
		highlightComment(line, styles, "#", "gray")
		highlightKeywords(line, styles, []string{"if", "then", "else", "fi", "for", "in", "do", "done", "case", "esac", "function"}, "deepskyblue", true)
		highlightStrings(line, styles, "gold")
		highlightNumbers(line, styles, "lightsalmon")
	case "yaml":
		highlightComment(line, styles, "#", "gray")
		highlightYAMLKeys(line, styles, "turquoise")
		highlightKeywords(line, styles, []string{"true", "false", "null", "yes", "no"}, "mediumpurple", true)
		highlightStrings(line, styles, "gold")
		highlightNumbers(line, styles, "lightsalmon")
	case "toml":
		highlightComment(line, styles, "#", "gray")
		highlightTOMLKeys(line, styles, "turquoise")
		highlightKeywords(line, styles, []string{"true", "false"}, "mediumpurple", true)
		highlightStrings(line, styles, "gold")
		highlightNumbers(line, styles, "lightsalmon")
	case "json":
		highlightJSONKeys(line, styles, "turquoise")
		highlightKeywords(line, styles, []string{"true", "false", "null"}, "mediumpurple", true)
		highlightStrings(line, styles, "gold")
		highlightNumbers(line, styles, "lightsalmon")
	default:
		highlightComment(line, styles, "#", "gray")
		highlightStrings(line, styles, "gold")
		highlightNumbers(line, styles, "lightsalmon")
	}
}

func detectSyntax(path string) string {
	path = strings.ToLower(path)
	switch {
	case strings.HasSuffix(path, ".go"):
		return "go"
	case strings.HasSuffix(path, ".sh"), strings.HasSuffix(path, ".bash"), strings.HasSuffix(path, ".zsh"):
		return "sh"
	case strings.HasSuffix(path, ".yaml"), strings.HasSuffix(path, ".yml"):
		return "yaml"
	case strings.HasSuffix(path, ".toml"):
		return "toml"
	case strings.HasSuffix(path, ".json"):
		return "json"
	default:
		return "plain"
	}
}

func applyInlineDiffHighlight(reference, line string, styles []textStyle) {
	refRunes := []rune(reference)
	lineRunes := []rune(line)
	refParts := make([]string, len(refRunes))
	lineParts := make([]string, len(lineRunes))
	for i, r := range refRunes {
		refParts[i] = string(r)
	}
	for i, r := range lineRunes {
		lineParts[i] = string(r)
	}

	matcher := difflib.NewMatcher(refParts, lineParts)
	for _, opcode := range matcher.GetOpCodes() {
		if opcode.Tag == 'e' {
			continue
		}
		for i := opcode.J1; i < opcode.J2 && i < len(styles); i++ {
			styles[i].bg = "maroon"
			if styles[i].fg == "white" {
				styles[i].fg = "white"
			}
		}
	}
}

func applySearchHighlight(line, keyword string, styles []textStyle) {
	lowerLine := strings.ToLower(line)
	lowerKeyword := strings.ToLower(keyword)
	if lowerKeyword == "" {
		return
	}

	start := 0
	for {
		index := strings.Index(lowerLine[start:], lowerKeyword)
		if index < 0 {
			return
		}
		absolute := start + index
		for i := absolute; i < absolute+len(lowerKeyword) && i < len(styles); i++ {
			styles[i].fg = "black"
			styles[i].bg = "yellow"
			styles[i].bold = true
		}
		start = absolute + len(lowerKeyword)
	}
}

func renderWithStyles(runes []rune, styles []textStyle) string {
	if len(runes) == 0 {
		return ""
	}

	var builder strings.Builder
	current := textStyle{}
	hasCurrent := false
	for i, r := range runes {
		style := styles[i]
		if !hasCurrent || style != current {
			builder.WriteString(styleTag(style))
			current = style
			hasCurrent = true
		}
		builder.WriteString(tview.Escape(string(r)))
	}
	builder.WriteString("[-:-:-]")
	return builder.String()
}

func styleTag(style textStyle) string {
	switch {
	case style.bg != "" && style.bold:
		return "[::b][" + style.fg + ":" + style.bg + "]"
	case style.bg != "":
		return "[" + style.fg + ":" + style.bg + "]"
	case style.bold:
		return "[" + style.fg + "::b]"
	default:
		return "[" + style.fg + "]"
	}
}

func highlightComment(line string, styles []textStyle, marker, color string) {
	index := strings.Index(line, marker)
	if index < 0 {
		return
	}
	for i := index; i < len(styles); i++ {
		styles[i].fg = color
	}
}

func highlightKeywords(line string, styles []textStyle, keywords []string, color string, bold bool) {
	for _, keyword := range keywords {
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(keyword) + `\b`)
		for _, index := range pattern.FindAllStringIndex(line, -1) {
			for i := index[0]; i < index[1] && i < len(styles); i++ {
				styles[i].fg = color
				styles[i].bold = bold
			}
		}
	}
}

func highlightStrings(line string, styles []textStyle, color string) {
	for _, index := range syntaxStringPattern.FindAllStringIndex(line, -1) {
		for i := index[0]; i < index[1] && i < len(styles); i++ {
			styles[i].fg = color
		}
	}
}

func highlightNumbers(line string, styles []textStyle, color string) {
	for _, index := range syntaxNumberPattern.FindAllStringIndex(line, -1) {
		for i := index[0]; i < index[1] && i < len(styles); i++ {
			styles[i].fg = color
		}
	}
}

func highlightYAMLKeys(line string, styles []textStyle, color string) {
	if strings.HasPrefix(strings.TrimSpace(line), "#") {
		return
	}
	index := strings.Index(line, ":")
	if index <= 0 {
		return
	}
	for i := 0; i < index && i < len(styles); i++ {
		if line[i] == ' ' || line[i] == '-' {
			continue
		}
		styles[i].fg = color
		styles[i].bold = true
	}
}

func highlightTOMLKeys(line string, styles []textStyle, color string) {
	if strings.HasPrefix(strings.TrimSpace(line), "[") {
		for i := 0; i < len(styles); i++ {
			styles[i].fg = "deepskyblue"
			styles[i].bold = true
		}
		return
	}

	index := strings.Index(line, "=")
	if index <= 0 {
		return
	}
	for i := 0; i < index && i < len(styles); i++ {
		if line[i] == ' ' {
			continue
		}
		styles[i].fg = color
		styles[i].bold = true
	}
}

func highlightJSONKeys(line string, styles []textStyle, color string) {
	for _, index := range syntaxJSONKeyPattern.FindAllStringIndex(line, -1) {
		for i := index[0]; i < index[1]-1 && i < len(styles); i++ {
			styles[i].fg = color
			styles[i].bold = true
		}
	}
}
