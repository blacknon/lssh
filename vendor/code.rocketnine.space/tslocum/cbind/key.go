package cbind

import (
	"errors"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// Modifier labels
const (
	LabelCtrl  = "ctrl"
	LabelAlt   = "alt"
	LabelMeta  = "meta"
	LabelShift = "shift"
)

// ErrInvalidKeyEvent is the error returned when encoding or decoding a key event fails.
var ErrInvalidKeyEvent = errors.New("invalid key event")

// UnifyEnterKeys is a flag that determines whether or not KPEnter (keypad
// enter) key events are interpreted as Enter key events. When enabled, Ctrl+J
// key events are also interpreted as Enter key events.
var UnifyEnterKeys = true

var fullKeyNames = map[string]string{
	"backspace2": "Backspace",
	"pgup":       "PageUp",
	"pgdn":       "PageDown",
	"esc":        "Escape",
}

var ctrlKeys = map[rune]tcell.Key{
	' ':  tcell.KeyCtrlSpace,
	'a':  tcell.KeyCtrlA,
	'b':  tcell.KeyCtrlB,
	'c':  tcell.KeyCtrlC,
	'd':  tcell.KeyCtrlD,
	'e':  tcell.KeyCtrlE,
	'f':  tcell.KeyCtrlF,
	'g':  tcell.KeyCtrlG,
	'h':  tcell.KeyCtrlH,
	'i':  tcell.KeyCtrlI,
	'j':  tcell.KeyCtrlJ,
	'k':  tcell.KeyCtrlK,
	'l':  tcell.KeyCtrlL,
	'm':  tcell.KeyCtrlM,
	'n':  tcell.KeyCtrlN,
	'o':  tcell.KeyCtrlO,
	'p':  tcell.KeyCtrlP,
	'q':  tcell.KeyCtrlQ,
	'r':  tcell.KeyCtrlR,
	's':  tcell.KeyCtrlS,
	't':  tcell.KeyCtrlT,
	'u':  tcell.KeyCtrlU,
	'v':  tcell.KeyCtrlV,
	'w':  tcell.KeyCtrlW,
	'x':  tcell.KeyCtrlX,
	'y':  tcell.KeyCtrlY,
	'z':  tcell.KeyCtrlZ,
	'\\': tcell.KeyCtrlBackslash,
	']':  tcell.KeyCtrlRightSq,
	'^':  tcell.KeyCtrlCarat,
	'_':  tcell.KeyCtrlUnderscore,
}

// Decode decodes a string as a key or combination of keys.
func Decode(s string) (mod tcell.ModMask, key tcell.Key, ch rune, err error) {
	if len(s) == 0 {
		return 0, 0, 0, ErrInvalidKeyEvent
	}

	// Special case for plus rune decoding
	if s[len(s)-1:] == "+" {
		key = tcell.KeyRune
		ch = '+'

		if len(s) == 1 {
			return mod, key, ch, nil
		} else if len(s) == 2 {
			return 0, 0, 0, ErrInvalidKeyEvent
		} else {
			s = s[:len(s)-2]
		}
	}

	split := strings.Split(s, "+")
DECODEPIECE:
	for _, piece := range split {
		// Decode modifiers
		pieceLower := strings.ToLower(piece)
		switch pieceLower {
		case LabelCtrl:
			mod |= tcell.ModCtrl
			continue
		case LabelAlt:
			mod |= tcell.ModAlt
			continue
		case LabelMeta:
			mod |= tcell.ModMeta
			continue
		case LabelShift:
			mod |= tcell.ModShift
			continue
		}

		// Decode key
		for shortKey, fullKey := range fullKeyNames {
			if pieceLower == strings.ToLower(fullKey) {
				pieceLower = shortKey
				break
			}
		}
		switch pieceLower {
		case "backspace":
			key = tcell.KeyBackspace2
			continue
		case "space", "spacebar":
			key = tcell.KeyRune
			ch = ' '
			continue
		}
		for k, keyName := range tcell.KeyNames {
			if pieceLower == strings.ToLower(strings.ReplaceAll(keyName, "-", "+")) {
				key = k
				if key < 0x80 {
					ch = rune(k)
				}
				continue DECODEPIECE
			}
		}

		// Decode rune
		if len(piece) > 1 {
			return 0, 0, 0, ErrInvalidKeyEvent
		}

		key = tcell.KeyRune
		ch = rune(piece[0])
	}

	if mod&tcell.ModCtrl != 0 {
		k, ok := ctrlKeys[unicode.ToLower(ch)]
		if ok {
			key = k
			if UnifyEnterKeys && key == ctrlKeys['j'] {
				key = tcell.KeyEnter
			} else if key < 0x80 {
				ch = rune(key)
			}
		}
	}

	return mod, key, ch, nil
}

// Encode encodes a key or combination of keys a string.
func Encode(mod tcell.ModMask, key tcell.Key, ch rune) (string, error) {
	var b strings.Builder
	var wrote bool

	if mod&tcell.ModCtrl != 0 {
		if key == tcell.KeyBackspace || key == tcell.KeyTab || key == tcell.KeyEnter {
			mod ^= tcell.ModCtrl
		} else {
			for _, ctrlKey := range ctrlKeys {
				if key == ctrlKey {
					mod ^= tcell.ModCtrl
					break
				}
			}
		}
	}

	if key != tcell.KeyRune {
		if UnifyEnterKeys && key == ctrlKeys['j'] {
			key = tcell.KeyEnter
		} else if key < 0x80 {
			ch = rune(key)
		}
	}

	// Encode modifiers
	if mod&tcell.ModCtrl != 0 {
		b.WriteString(upperFirst(LabelCtrl))
		wrote = true
	}
	if mod&tcell.ModAlt != 0 {
		if wrote {
			b.WriteRune('+')
		}
		b.WriteString(upperFirst(LabelAlt))
		wrote = true
	}
	if mod&tcell.ModMeta != 0 {
		if wrote {
			b.WriteRune('+')
		}
		b.WriteString(upperFirst(LabelMeta))
		wrote = true
	}
	if mod&tcell.ModShift != 0 {
		if wrote {
			b.WriteRune('+')
		}
		b.WriteString(upperFirst(LabelShift))
		wrote = true
	}

	if key == tcell.KeyRune && ch == ' ' {
		if wrote {
			b.WriteRune('+')
		}
		b.WriteString("Space")
	} else if key != tcell.KeyRune {
		// Encode key
		keyName := tcell.KeyNames[key]
		if keyName == "" {
			return "", ErrInvalidKeyEvent
		}
		keyName = strings.ReplaceAll(keyName, "-", "+")
		fullKeyName := fullKeyNames[strings.ToLower(keyName)]
		if fullKeyName != "" {
			keyName = fullKeyName
		}

		if wrote {
			b.WriteRune('+')
		}
		b.WriteString(keyName)
	} else {
		// Encode rune
		if wrote {
			b.WriteRune('+')
		}
		b.WriteRune(ch)
	}

	return b.String(), nil
}

func upperFirst(s string) string {
	if len(s) <= 1 {
		return strings.ToUpper(s)
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
