package ldif

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"
)

const defaultFoldWidth = 76

func EncodeLine(name string, value Value) string {
	var raw string
	useBase64 := value.Base64
	if value.Bytes != nil {
		raw = string(value.Bytes)
		useBase64 = true
	} else {
		raw = value.Text
		useBase64 = useBase64 || shouldBase64(raw)
	}
	if useBase64 {
		src := []byte(raw)
		if value.Bytes != nil {
			src = value.Bytes
		}
		return fold(name+":: "+base64.StdEncoding.EncodeToString(src), defaultFoldWidth)
	}
	return fold(name+": "+raw, defaultFoldWidth)
}

func shouldBase64(v string) bool {
	if v == "" {
		return false
	}
	if strings.HasPrefix(v, " ") || strings.HasPrefix(v, ":") || strings.HasPrefix(v, "<") {
		return true
	}
	if strings.HasSuffix(v, " ") {
		return true
	}
	for _, r := range v {
		if r == 0 || r == '\n' || r == '\r' || r < 0x20 || r >= 0x7f {
			return true
		}
	}
	return !utf8.ValidString(v)
}

func fold(line string, width int) string {
	if width <= 0 || len(line) <= width {
		return line
	}
	var b strings.Builder
	for len(line) > width {
		b.WriteString(line[:width])
		b.WriteByte('\n')
		b.WriteByte(' ')
		line = line[width:]
		width = defaultFoldWidth - 1
	}
	b.WriteString(line)
	return b.String()
}
