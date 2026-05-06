package generator

import (
	"strings"

	"github.com/yoursevg/LDIFGenerator/internal/schema"
)

// AttributeSupportPolicy controls which schema constructions are safe to emit.
//
// Some LDAP servers can parse a schema attributeType but still reject values for
// attributes that use not-yet-implemented syntaxes or matching rules. Keep those
// constructions disabled here until the target server supports them.
type AttributeSupportPolicy struct {
	DisabledSyntaxes   map[string]bool
	DisabledEqualities map[string]bool
	DisabledOrderings  map[string]bool
	DisabledSubstrs    map[string]bool

	EnabledSyntaxes   map[string]bool
	EnabledEqualities map[string]bool
	EnabledOrderings  map[string]bool
	EnabledSubstrs    map[string]bool
}

func DefaultAttributeSupportPolicy() AttributeSupportPolicy {
	return AttributeSupportPolicy{
		DisabledSyntaxes: map[string]bool{
			"1.3.6.1.4.1.1466.115.121.1.5": true,
			"1.3.6.1.4.1.4203.1.1.2":       true,
		},
		DisabledEqualities: map[string]bool{
			"1.3.6.1.4.1.4203.1.2.2":     true,
			"integerfirstcomponentmatch": true,
		},
		DisabledOrderings: map[string]bool{
			"objectidentifierorderingmatch": true,
		},
		DisabledSubstrs: map[string]bool{
			"caseexactia5substringsmatch":    true,
			"caseignorelistsubstringsmatch":  true,
			"numericstringsubstringsmatch":   true,
			"telephonenumbersubstringsmatch": true,
		},
	}
}

func (p AttributeSupportPolicy) Allows(attr schema.AttributeType) bool {
	if constructionDisabled(p.DisabledSyntaxes, p.EnabledSyntaxes, normalizeSyntaxConstruction(attr.Syntax)) {
		return false
	}
	if constructionDisabled(p.DisabledEqualities, p.EnabledEqualities, normalizeRuleConstruction(attr.Equality)) {
		return false
	}
	if constructionDisabled(p.DisabledOrderings, p.EnabledOrderings, normalizeRuleConstruction(attr.Ordering)) {
		return false
	}
	if constructionDisabled(p.DisabledSubstrs, p.EnabledSubstrs, normalizeRuleConstruction(attr.Substr)) {
		return false
	}
	return true
}

func (p AttributeSupportPolicy) EnableSyntaxes(values ...string) AttributeSupportPolicy {
	p.EnabledSyntaxes = setConstructions(p.EnabledSyntaxes, normalizeSyntaxConstruction, values...)
	return p
}

func (p AttributeSupportPolicy) DisableSyntaxes(values ...string) AttributeSupportPolicy {
	p.DisabledSyntaxes, p.EnabledSyntaxes = moveConstructions(p.DisabledSyntaxes, p.EnabledSyntaxes, normalizeSyntaxConstruction, values...)
	return p
}

func (p AttributeSupportPolicy) EnableEqualities(values ...string) AttributeSupportPolicy {
	p.EnabledEqualities = setConstructions(p.EnabledEqualities, normalizeRuleConstruction, values...)
	return p
}

func (p AttributeSupportPolicy) DisableEqualities(values ...string) AttributeSupportPolicy {
	p.DisabledEqualities, p.EnabledEqualities = moveConstructions(p.DisabledEqualities, p.EnabledEqualities, normalizeRuleConstruction, values...)
	return p
}

func (p AttributeSupportPolicy) EnableOrderings(values ...string) AttributeSupportPolicy {
	p.EnabledOrderings = setConstructions(p.EnabledOrderings, normalizeRuleConstruction, values...)
	return p
}

func (p AttributeSupportPolicy) DisableOrderings(values ...string) AttributeSupportPolicy {
	p.DisabledOrderings, p.EnabledOrderings = moveConstructions(p.DisabledOrderings, p.EnabledOrderings, normalizeRuleConstruction, values...)
	return p
}

func (p AttributeSupportPolicy) EnableSubstrs(values ...string) AttributeSupportPolicy {
	p.EnabledSubstrs = setConstructions(p.EnabledSubstrs, normalizeRuleConstruction, values...)
	return p
}

func (p AttributeSupportPolicy) DisableSubstrs(values ...string) AttributeSupportPolicy {
	p.DisabledSubstrs, p.EnabledSubstrs = moveConstructions(p.DisabledSubstrs, p.EnabledSubstrs, normalizeRuleConstruction, values...)
	return p
}

func constructionDisabled(disabled map[string]bool, enabled map[string]bool, value string) bool {
	if value == "" {
		return false
	}
	if enabled[value] {
		return false
	}
	return disabled[value]
}

func setConstructions(dst map[string]bool, normalize func(string) string, values ...string) map[string]bool {
	if dst == nil {
		dst = map[string]bool{}
	}
	for _, value := range values {
		if normalized := normalize(value); normalized != "" {
			dst[normalized] = true
		}
	}
	return dst
}

func moveConstructions(dst map[string]bool, src map[string]bool, normalize func(string) string, values ...string) (map[string]bool, map[string]bool) {
	if dst == nil {
		dst = map[string]bool{}
	}
	for _, value := range values {
		normalized := normalize(value)
		if normalized == "" {
			continue
		}
		dst[normalized] = true
		delete(src, normalized)
	}
	return dst, src
}

func normalizeSyntaxConstruction(value string) string {
	return syntaxOID(value)
}

func normalizeRuleConstruction(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
