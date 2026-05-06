package validation

import (
	"strings"
	"testing"

	"github.com/yoursevg/LDIFGenerator/internal/ldif"
	"github.com/yoursevg/LDIFGenerator/internal/schema"
)

func TestValidatorAllowsAttributeAliases(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(`
attributeTypes: ( 2.5.4.0 NAME 'objectClass' )
attributeTypes: ( 2.5.4.3 NAME 'cn' )
attributeTypes: ( 1.2.840.113556.1.2.226 NAME ( 'administratorContactInfo' 'adminDescription' ) )
objectClasses: ( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )
objectClasses: ( 1.2.643.4.38.2.2.2 NAME 'serviceUser' SUP top STRUCTURAL MUST cn MAY adminDescription )
`))
	if err != nil {
		t.Fatal(err)
	}
	rec := ldif.NewRecord("cn=svc-0000001,dc=example,dc=com")
	rec.Add("objectClass", "top", "serviceUser")
	rec.Add("cn", "svc-0000001")
	rec.Add("administratorContactInfo", "generated")
	if err := New(s).ValidateRecord(rec, []string{"serviceUser"}, true); err != nil {
		t.Fatal(err)
	}
}

func TestValidatorRejectsNoUserModificationAttributes(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(`
attributeTypes: ( 2.5.4.0 NAME 'objectClass' )
attributeTypes: ( 2.5.4.3 NAME 'cn' )
attributeTypes: ( 2.5.4.41 NAME 'name' NO-USER-MODIFICATION )
objectClasses: ( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )
objectClasses: ( 1.2.3.4 NAME 'exampleUser' SUP top STRUCTURAL MUST ( cn $ name ) )
`))
	if err != nil {
		t.Fatal(err)
	}
	rec := ldif.NewRecord("cn=user,dc=example,dc=com")
	rec.Add("objectClass", "top", "exampleUser")
	rec.Add("cn", "user")
	if err := New(s).ValidateRecord(rec, []string{"exampleUser"}, true); err != nil {
		t.Fatalf("NO-USER-MODIFICATION MUST should not be required from client: %v", err)
	}
	rec.Add("name", "user")
	if err := New(s).ValidateRecord(rec, []string{"exampleUser"}, true); err == nil {
		t.Fatal("expected NO-USER-MODIFICATION attribute to be rejected")
	}
}
