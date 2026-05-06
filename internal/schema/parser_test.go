package schema

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestParseSchemaAliasesAndInheritance(t *testing.T) {
	input := `
attributeTypes: ( 2.5.4.3 NAME ( 'cn' 'commonName' ) SUP name )
attributeTypes: ( 2.5.4.4 NAME 'sn' SUP name )
objectClasses: ( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )
objectClasses: ( 2.5.6.6 NAME 'person' SUP top STRUCTURAL MUST ( sn $ cn ) MAY ( userPassword $ telephoneNumber $ description ) )
`
	s, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Attribute("commonName"); !ok {
		t.Fatal("alias commonName was not indexed")
	}
	resolved, err := s.ResolveObjectClasses([]string{"person"})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, resolved.Must, "objectClass")
	assertContains(t, resolved.Must, "cn")
	assertContains(t, resolved.Must, "sn")
	assertContains(t, resolved.May, "description")
}

func TestParseFoldedSchemaLine(t *testing.T) {
	input := "objectClasses: ( 1.2 NAME 'x' STRUCTURAL MUST ( cn $\n sn ) MAY description )\nattributeTypes: ( 2.5.4.3 NAME 'cn' SUP name )\nattributeTypes: ( 2.5.4.4 NAME 'sn' SUP name )\n"
	s, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	oc, ok := s.ObjectClass("x")
	if !ok {
		t.Fatal("objectClass x missing")
	}
	assertContains(t, oc.Must, "sn")
}

func TestParseOpenLDAPConfigSchema(t *testing.T) {
	input := `
olcAttributeTypes: {0}( 2.5.4.42 NAME 'givenName' SUP name )
olcAttributeTypes: {1}( 2.5.4.3 NAME 'cn' SUP name )
olcAttributeTypes: {2}( 2.5.4.4 NAME 'sn' SUP name )
olcAttributeTypes: {3}( 2.5.4.0 NAME 'objectClass' )
olcObjectClasses: {0}( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )
olcObjectClasses: {1}( 2.16.840.1.113730.3.2.2 NAME 'inetOrgPerson' SUP top STRUCTURAL MUST ( cn $ sn ) MAY givenName )
`
	s, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Attribute("givenName"); !ok {
		t.Fatalf("givenName was not indexed, warnings: %#v", s.Warnings)
	}
	resolved, err := s.ResolveObjectClasses([]string{"inetOrgPerson"})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, resolved.May, "givenName")
}

func TestParseBase64EncodedSchemaValue(t *testing.T) {
	def := "( 2.5.4.42 NAME 'givenName' SUP name )"
	input := fmt.Sprintf("attributeTypes:: %s\n", base64.StdEncoding.EncodeToString([]byte(def)))
	s, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Attribute("givenName"); !ok {
		t.Fatalf("givenName was not indexed from base64 schema value, warnings: %#v", s.Warnings)
	}
}

func TestParseGivenNameFromExampleStyleMultilineDefinition(t *testing.T) {
	input := `
attributeTypes: ( 2.5.4.42
  NAME 'givenName'
  DESC 'Contains name string that are the part of a person's name that is not their surname.'
  SUP name
  SINGLE-VALUE
  X-ORIGIN 'RFC 4519' )
`
	s, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Attribute("givenName"); !ok {
		t.Fatalf("givenName was not indexed, warnings: %#v", s.Warnings)
	}
}

func TestResolveAttributeTypeInheritance(t *testing.T) {
	input := `
attributeTypes: ( 2.5.4.41 NAME 'name' SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 NO-USER-MODIFICATION )
attributeTypes: ( 2.5.4.3 NAME 'cn' SUP name )
attributeTypes: ( 2.5.4.49 NAME 'distinguishedName' SYNTAX 1.3.6.1.4.1.1466.115.121.1.12 )
attributeTypes: ( 2.5.4.34 NAME 'seeAlso' SUP distinguishedName )
`
	s, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	attr, ok := s.ResolveAttributeType("seeAlso")
	if !ok {
		t.Fatal("seeAlso was not indexed")
	}
	if attr.Syntax != "1.3.6.1.4.1.1466.115.121.1.12" {
		t.Fatalf("seeAlso syntax = %q, want inherited DN syntax", attr.Syntax)
	}
	attr, ok = s.ResolveAttributeType("cn")
	if !ok {
		t.Fatal("cn was not indexed")
	}
	if attr.Syntax != "1.3.6.1.4.1.1466.115.121.1.15" {
		t.Fatalf("cn syntax = %q, want inherited Directory String syntax", attr.Syntax)
	}
	if attr.NoUserMod {
		t.Fatal("cn must not inherit NO-USER-MODIFICATION from name")
	}
}

func assertContains(t *testing.T, got []string, want string) {
	t.Helper()
	for _, v := range got {
		if NormalizeName(v) == NormalizeName(want) {
			return
		}
	}
	t.Fatalf("%q not found in %#v", want, got)
}
