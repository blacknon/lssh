package psrp

import (
	"encoding/xml"
	"strings"
)

func RenderSerializedText(raw []byte) string {
	decoder := xml.NewDecoder(strings.NewReader(string(raw)))
	var builder strings.Builder
	inText := false

	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}

		switch typed := token.(type) {
		case xml.StartElement:
			switch typed.Name.Local {
			case "S", "ToString":
				inText = true
			}
		case xml.EndElement:
			switch typed.Name.Local {
			case "S", "ToString":
				inText = false
			}
		case xml.CharData:
			if inText {
				builder.WriteString(string(typed))
			}
		}
	}

	text := strings.TrimSpace(builder.String())
	if text != "" {
		return text
	}
	return strings.TrimSpace(string(raw))
}
