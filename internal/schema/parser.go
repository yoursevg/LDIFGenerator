package schema

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

func ParseFiles(paths []string) (*Schema, error) {
	out := New()
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		parsed, parseErr := Parse(f)
		closeErr := f.Close()
		if parseErr != nil {
			return nil, fmt.Errorf("%s: %w", path, parseErr)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		for _, attr := range parsed.AttributeTypes {
			out.AddAttributeType(attr)
		}
		for _, oc := range parsed.ObjectClasses {
			out.AddObjectClass(oc)
		}
		out.Warnings = append(out.Warnings, parsed.Warnings...)
	}
	return out, nil
}

func Parse(r io.Reader) (*Schema, error) {
	lines, err := unfold(r)
	if err != nil {
		return nil, err
	}
	s := New()
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = normalizeLDIFKey(key)
		value, err := decodeLDIFValue(value)
		if err != nil {
			s.Warnings = append(s.Warnings, err.Error())
			continue
		}
		switch key {
		case "attributetypes", "olcattributetypes":
			attr, err := parseAttributeType(value)
			if err != nil {
				s.Warnings = append(s.Warnings, err.Error())
				continue
			}
			s.AddAttributeType(attr)
		case "objectclasses", "olcobjectclasses":
			oc, err := parseObjectClass(value)
			if err != nil {
				s.Warnings = append(s.Warnings, err.Error())
				continue
			}
			s.AddObjectClass(oc)
		}
	}
	return s, nil
}

func normalizeLDIFKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	if base, _, ok := strings.Cut(key, ";"); ok {
		return base
	}
	return key
}

func decodeLDIFValue(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, ":") {
		return value, nil
	}
	value = strings.TrimSpace(strings.TrimPrefix(value, ":"))
	if strings.HasPrefix(value, "<") {
		return "", fmt.Errorf("schema value loaded from URL is not supported: %q", value)
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("invalid base64 schema value: %w", err)
	}
	return string(decoded), nil
}

func unfold(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var out []string
	var cur strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, " ") {
			appendContinuation(&cur, line)
			continue
		}
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
		cur.WriteString(line)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out, scanner.Err()
}

func appendContinuation(cur *strings.Builder, line string) {
	trimmed := strings.TrimSpace(line)
	if cur.Len() == 0 {
		cur.WriteString(trimmed)
		return
	}
	current := cur.String()
	key, _, _ := strings.Cut(current, ":")
	key = normalizeLDIFKey(key)
	if strings.HasSuffix(key, ":") {
		cur.WriteString(strings.TrimPrefix(line, " "))
		return
	}
	if key == "attributetypes" || key == "objectclasses" || key == "olcattributetypes" || key == "olcobjectclasses" {
		cur.WriteByte(' ')
		cur.WriteString(trimmed)
		return
	}
	cur.WriteString(strings.TrimPrefix(line, " "))
}

func parseAttributeType(def string) (AttributeType, error) {
	def = trimSchemaDefinition(def)
	tokens := tokenize(def)
	if len(tokens) < 3 || tokens[0] != "(" {
		return AttributeType{}, fmt.Errorf("invalid attributeType definition: %q", def)
	}
	attr := AttributeType{OID: tokens[1]}
	for i := 2; i < len(tokens); i++ {
		switch strings.ToUpper(tokens[i]) {
		case "NAME":
			names, next := parseNames(tokens, i+1)
			attr.Names = names
			i = next - 1
		case "DESC":
			if i+1 < len(tokens) {
				attr.Description = tokens[i+1]
				i++
			}
		case "SUP":
			if i+1 < len(tokens) {
				attr.SUP = tokens[i+1]
				i++
			}
		case "EQUALITY":
			attr.Equality, i = nextValue(tokens, i)
		case "ORDERING":
			attr.Ordering, i = nextValue(tokens, i)
		case "SUBSTR":
			attr.Substr, i = nextValue(tokens, i)
		case "SYNTAX":
			attr.Syntax, i = nextValue(tokens, i)
		case "SINGLE-VALUE":
			attr.SingleValue = true
		case "NO-USER-MODIFICATION":
			attr.NoUserMod = true
		case "USAGE":
			attr.Usage, i = nextValue(tokens, i)
		}
	}
	if attr.OID == "" || len(attr.Names) == 0 {
		return AttributeType{}, fmt.Errorf("attributeType missing OID or NAME: %q", def)
	}
	return attr, nil
}

func parseObjectClass(def string) (ObjectClass, error) {
	def = trimSchemaDefinition(def)
	tokens := tokenize(def)
	if len(tokens) < 3 || tokens[0] != "(" {
		return ObjectClass{}, fmt.Errorf("invalid objectClass definition: %q", def)
	}
	oc := ObjectClass{OID: tokens[1], Kind: ObjectClassStructural}
	for i := 2; i < len(tokens); i++ {
		switch strings.ToUpper(tokens[i]) {
		case "NAME":
			names, next := parseNames(tokens, i+1)
			oc.Names = names
			i = next - 1
		case "DESC":
			if i+1 < len(tokens) {
				oc.Description = tokens[i+1]
				i++
			}
		case "SUP":
			refs, next := parseList(tokens, i+1)
			oc.SUP = refs
			i = next - 1
		case "ABSTRACT":
			oc.Kind = ObjectClassAbstract
		case "STRUCTURAL":
			oc.Kind = ObjectClassStructural
		case "AUXILIARY":
			oc.Kind = ObjectClassAuxiliary
		case "MUST":
			values, next := parseList(tokens, i+1)
			oc.Must = values
			i = next - 1
		case "MAY":
			values, next := parseList(tokens, i+1)
			oc.May = values
			i = next - 1
		}
	}
	if oc.OID == "" || len(oc.Names) == 0 {
		return ObjectClass{}, fmt.Errorf("objectClass missing OID or NAME: %q", def)
	}
	return oc, nil
}

func trimSchemaDefinition(def string) string {
	def = strings.TrimSpace(def)
	if i := strings.Index(def, "("); i > 0 {
		return strings.TrimSpace(def[i:])
	}
	return def
}

func nextValue(tokens []string, i int) (string, int) {
	if i+1 >= len(tokens) {
		return "", i
	}
	return tokens[i+1], i + 1
}

func parseNames(tokens []string, start int) ([]string, int) {
	return parseList(tokens, start)
}

func parseList(tokens []string, start int) ([]string, int) {
	if start >= len(tokens) {
		return nil, start
	}
	if tokens[start] != "(" {
		return []string{tokens[start]}, start + 1
	}
	var values []string
	i := start + 1
	for ; i < len(tokens); i++ {
		if tokens[i] == ")" {
			return values, i + 1
		}
		if tokens[i] == "$" {
			continue
		}
		values = append(values, tokens[i])
	}
	return values, i
}

func tokenize(s string) []string {
	s = strings.NewReplacer("‘", "'", "’", "'").Replace(s)
	var tokens []string
	for i := 0; i < len(s); {
		r := rune(s[i])
		if unicode.IsSpace(r) {
			i++
			continue
		}
		if s[i] == '(' || s[i] == ')' || s[i] == '$' {
			tokens = append(tokens, string(s[i]))
			i++
			continue
		}
		if s[i] == '\'' {
			i++
			start := i
			for i < len(s) {
				if s[i] == '\'' && isQuoteTerminator(s, i) {
					break
				}
				i++
			}
			tokens = append(tokens, s[start:i])
			if i < len(s) {
				i++
			}
			continue
		}
		start := i
		for i < len(s) && !unicode.IsSpace(rune(s[i])) && !strings.ContainsRune("()$", rune(s[i])) {
			i++
		}
		tokens = append(tokens, s[start:i])
	}
	return tokens
}

func isQuoteTerminator(s string, i int) bool {
	if i+1 >= len(s) {
		return true
	}
	next := rune(s[i+1])
	return unicode.IsSpace(next) || next == ')' || next == '(' || next == '$'
}
