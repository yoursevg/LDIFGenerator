package validation

import (
	"fmt"
	"regexp"

	"github.com/yoursevg/LDIFGenerator/internal/ldif"
	"github.com/yoursevg/LDIFGenerator/internal/schema"
)

var dnPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9-]*=.+(,[a-zA-Z][a-zA-Z0-9-]*=.+)*$`)

type Validator struct {
	schema *schema.Schema
}

func New(s *schema.Schema) *Validator {
	return &Validator{schema: s}
}

func (v *Validator) ValidateRecord(record ldif.Record, objectClasses []string, strict bool) error {
	if !dnPattern.MatchString(record.DN) {
		return fmt.Errorf("invalid DN %q", record.DN)
	}
	resolved, err := v.schema.ResolveObjectClasses(objectClasses)
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	var required [][]string
	for _, name := range resolved.Must {
		keys := v.attributeKeys(name)
		required = append(required, keys)
		for _, key := range keys {
			allowed[key] = true
		}
	}
	for _, name := range resolved.May {
		for _, key := range v.attributeKeys(name) {
			allowed[key] = true
		}
	}
	allowed["objectclass"] = true
	seen := map[string]bool{"objectclass": true}
	for _, attr := range record.Attributes {
		key := schema.NormalizeName(attr.Name)
		if strict {
			if _, ok := v.schema.Attribute(key); !ok && key != "objectclass" {
				return fmt.Errorf("attribute %q is not defined in schema", attr.Name)
			}
			if !allowed[key] {
				return fmt.Errorf("attribute %q is not allowed by objectClass set %v", attr.Name, objectClasses)
			}
		}
		if len(attr.Values) > 0 {
			seen[key] = true
		}
	}
	for _, keys := range required {
		if !hasAnySeen(seen, keys) {
			return fmt.Errorf("required attribute %q is missing for DN %q", keys[0], record.DN)
		}
	}
	return nil
}

func hasAnySeen(seen map[string]bool, keys []string) bool {
	for _, key := range keys {
		if seen[key] {
			return true
		}
	}
	return false
}

func (v *Validator) attributeKeys(name string) []string {
	keys := []string{schema.NormalizeName(name)}
	if attr, ok := v.schema.Attribute(name); ok {
		for _, alias := range attr.Names {
			keys = append(keys, schema.NormalizeName(alias))
		}
		if attr.OID != "" {
			keys = append(keys, schema.NormalizeName(attr.OID))
		}
	}
	return keys
}
