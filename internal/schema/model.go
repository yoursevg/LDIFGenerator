package schema

import "strings"

type AttributeType struct {
	OID         string   `json:"oid"`
	Names       []string `json:"names"`
	Description string   `json:"description,omitempty"`
	SUP         string   `json:"sup,omitempty"`
	Equality    string   `json:"equality,omitempty"`
	Ordering    string   `json:"ordering,omitempty"`
	Substr      string   `json:"substr,omitempty"`
	Syntax      string   `json:"syntax,omitempty"`
	SingleValue bool     `json:"singleValue,omitempty"`
	NoUserMod   bool     `json:"noUserMod,omitempty"`
	Usage       string   `json:"usage,omitempty"`
}

func (a AttributeType) PrimaryName() string {
	if len(a.Names) == 0 {
		return a.OID
	}
	return a.Names[0]
}

type ObjectClassKind string

const (
	ObjectClassAbstract   ObjectClassKind = "ABSTRACT"
	ObjectClassStructural ObjectClassKind = "STRUCTURAL"
	ObjectClassAuxiliary  ObjectClassKind = "AUXILIARY"
)

type ObjectClass struct {
	OID         string          `json:"oid"`
	Names       []string        `json:"names"`
	Description string          `json:"description,omitempty"`
	SUP         []string        `json:"sup,omitempty"`
	Kind        ObjectClassKind `json:"kind"`
	Must        []string        `json:"must,omitempty"`
	May         []string        `json:"may,omitempty"`
}

func (o ObjectClass) PrimaryName() string {
	if len(o.Names) == 0 {
		return o.OID
	}
	return o.Names[0]
}

type Schema struct {
	AttributeTypes map[string]AttributeType `json:"attributeTypes"`
	ObjectClasses  map[string]ObjectClass   `json:"objectClasses"`
	Warnings       []string                 `json:"warnings,omitempty"`
}

func New() *Schema {
	return &Schema{
		AttributeTypes: make(map[string]AttributeType),
		ObjectClasses:  make(map[string]ObjectClass),
	}
}

func NormalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func (s *Schema) AddAttributeType(attr AttributeType) {
	for _, name := range attr.Names {
		s.AttributeTypes[NormalizeName(name)] = attr
	}
	if attr.OID != "" {
		s.AttributeTypes[NormalizeName(attr.OID)] = attr
	}
}

func (s *Schema) AddObjectClass(oc ObjectClass) {
	for _, name := range oc.Names {
		s.ObjectClasses[NormalizeName(name)] = oc
	}
	if oc.OID != "" {
		s.ObjectClasses[NormalizeName(oc.OID)] = oc
	}
}

func (s *Schema) Attribute(name string) (AttributeType, bool) {
	attr, ok := s.AttributeTypes[NormalizeName(name)]
	return attr, ok
}

func (s *Schema) ObjectClass(name string) (ObjectClass, bool) {
	oc, ok := s.ObjectClasses[NormalizeName(name)]
	return oc, ok
}
