package generator

import (
	"context"
	"math/rand"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/yoursevg/LDIFGenerator/internal/schema"
)

func TestGeneratorStreamsRecords(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(testSchema))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "generated-*.ldif")
	if err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	cfg := DefaultConfig()
	cfg.Count = 50
	cfg.OutputPath = out.Name()
	cfg.BatchSize = 10
	g := New(s)
	var progress int
	report, err := g.Generate(context.Background(), cfg, func(done, _ int) { progress = done })
	if err != nil {
		t.Fatal(err)
	}
	if progress != cfg.Count {
		t.Fatalf("progress = %d, want %d", progress, cfg.Count)
	}
	if report.Records < cfg.Count {
		t.Fatalf("records = %d, want at least %d", report.Records, cfg.Count)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "objectClass: inetOrgPerson") {
		t.Fatalf("generated LDIF missing users:\n%s", string(data[:min(len(data), 500)]))
	}
}

func TestGeneratorDoesNotDuplicateSingleValueManager(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(testSchema))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "generated-*.ldif")
	if err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	cfg := DefaultConfig()
	cfg.Count = 100
	cfg.OutputPath = out.Name()
	cfg.OptionalFillPercent = 100
	cfg.SelectedAttributes = map[string]bool{"manager": true}
	cfg.Tree.GroupPercent = 0
	cfg.Tree.ComputerPercent = 0
	cfg.Tree.ServicePercent = 0
	cfg.Tree.PrivilegedPercent = 0
	cfg.Relationships.ManagersPercent = 100
	report, err := New(s).Generate(context.Background(), cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Records == 0 {
		t.Fatal("expected generated records")
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range strings.Split(string(data), "\n\n") {
		if strings.Count(entry, "\nmanager: ") > 1 {
			t.Fatalf("entry has duplicate manager values:\n%s", entry)
		}
	}
}

func TestBuildPlanWritesGroupsAfterMemberEntries(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Count = 100
	cfg.Tree.GroupPercent = 20
	cfg.Tree.ComputerPercent = 10
	cfg.Tree.ServicePercent = 10
	cfg.Tree.PrivilegedPercent = 10
	plan := BuildPlan(cfg, rand.New(rand.NewSource(1)))
	seenGroup := false
	for _, typ := range plan {
		if typ == EntryTypeGroup {
			seenGroup = true
			continue
		}
		if seenGroup {
			t.Fatalf("non-group entry %q generated after first group in plan: %#v", typ, plan)
		}
	}
}

func TestNestedGroupsReferenceEarlierGroupRecords(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Count = 100
	cfg.Tree.GroupPercent = 30
	cfg.Tree.ComputerPercent = 0
	cfg.Tree.ServicePercent = 0
	cfg.Tree.PrivilegedPercent = 0
	cfg.Relationships.NestedGroupsPercent = 100
	rng := rand.New(rand.NewSource(1))
	plan := BuildPlan(cfg, rng)
	rel := BuildRelationships(cfg, plan, rng)
	groupOrder := map[string]int{}
	for i, typ := range plan {
		if typ != EntryTypeGroup {
			continue
		}
		ec := baseEntryContext(cfg, typ, i, Relationships{}, rand.New(rand.NewSource(2)))
		groupOrder[ec.DN] = i
	}
	for parent, members := range rel.GroupMembers {
		parentIndex, ok := groupOrder[parent]
		if !ok {
			continue
		}
		for _, member := range members {
			memberIndex, ok := groupOrder[member]
			if ok && memberIndex > parentIndex {
				t.Fatalf("nested group parent %q at %d references later group %q at %d", parent, parentIndex, member, memberIndex)
			}
		}
	}
}

func TestAllUsersGroupCountAddsHumanUsersToLeadingGroups(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Count = 100
	cfg.Tree.GroupPercent = 10
	cfg.Tree.ComputerPercent = 10
	cfg.Tree.ServicePercent = 10
	cfg.Tree.PrivilegedPercent = 10
	cfg.Relationships.UsersInGroupsPercent = 0
	cfg.Relationships.AllUsersGroupCount = 2
	rng := rand.New(rand.NewSource(1))
	plan := BuildPlan(cfg, rng)
	rel := BuildRelationships(cfg, plan, rng)
	var groups, humanUsers, computersAndServices []string
	for i, typ := range plan {
		ec := baseEntryContext(cfg, typ, i, Relationships{}, rand.New(rand.NewSource(2)))
		switch typ {
		case EntryTypeGroup:
			groups = append(groups, ec.DN)
		case EntryTypeUser, EntryTypePrivileged:
			humanUsers = append(humanUsers, ec.DN)
		case EntryTypeComputer, EntryTypeService:
			computersAndServices = append(computersAndServices, ec.DN)
		}
	}
	if len(groups) < 2 || len(humanUsers) == 0 {
		t.Fatalf("test plan missing groups or users: groups=%d users=%d", len(groups), len(humanUsers))
	}
	for _, groupDN := range groups[:2] {
		for _, userDN := range humanUsers {
			if !containsDN(rel.GroupMembers[groupDN], userDN) {
				t.Fatalf("group %q is missing user %q", groupDN, userDN)
			}
			if !containsDN(rel.UserGroups[userDN], groupDN) {
				t.Fatalf("user %q is missing memberOf %q", userDN, groupDN)
			}
		}
		for _, dn := range computersAndServices {
			if containsDN(rel.GroupMembers[groupDN], dn) {
				t.Fatalf("all-users group %q unexpectedly contains non-human entry %q", groupDN, dn)
			}
		}
	}
}

func TestGeneratorWritesForcedGroupMembersWhenMemberIsMay(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(testSchema))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "generated-*.ldif")
	if err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	cfg := DefaultConfig()
	cfg.Count = 50
	cfg.OutputPath = out.Name()
	cfg.OptionalFillPercent = 0
	cfg.Tree.GroupPercent = 10
	cfg.Tree.ComputerPercent = 0
	cfg.Tree.ServicePercent = 0
	cfg.Tree.PrivilegedPercent = 10
	cfg.Relationships.UsersInGroupsPercent = 0
	cfg.Relationships.AllUsersGroupCount = 1
	if _, err := New(s).Generate(context.Background(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	entries := strings.Split(string(data), "\n\n")
	for _, entry := range entries {
		if strings.Contains(entry, "objectClass: groupOfNames") && !strings.Contains(entry, "\nmember: ") {
			t.Fatalf("group entry missing forced member values:\n%s", entry)
		}
	}
}

func TestGeneratorSkipsNoUserModificationAttributes(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(testSchema))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "generated-*.ldif")
	if err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	cfg := DefaultConfig()
	cfg.Count = 20
	cfg.OutputPath = out.Name()
	cfg.OptionalFillPercent = 100
	cfg.SelectedAttributes = map[string]bool{"name": true, "memberOf": true}
	cfg.Tree.GroupPercent = 10
	cfg.Tree.ComputerPercent = 0
	cfg.Tree.ServicePercent = 0
	cfg.Tree.PrivilegedPercent = 0
	cfg.Relationships.UsersInGroupsPercent = 100
	if _, err := New(s).Generate(context.Background(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "\nname: ") {
		t.Fatalf("generated LDIF contains NO-USER-MODIFICATION name attribute:\n%s", text)
	}
	if strings.Contains(text, "\nmemberOf: ") {
		t.Fatalf("generated LDIF contains NO-USER-MODIFICATION memberOf attribute:\n%s", text)
	}
}

func TestGeneratorSkipsUserPassword(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(testSchema))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "generated-*.ldif")
	if err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	cfg := DefaultConfig()
	cfg.Count = 20
	cfg.OutputPath = out.Name()
	cfg.OptionalFillPercent = 100
	cfg.SelectedAttributes = map[string]bool{"userpassword": true}
	cfg.Tree.GroupPercent = 0
	cfg.Tree.ComputerPercent = 0
	cfg.Tree.ServicePercent = 0
	cfg.Tree.PrivilegedPercent = 0
	if _, err := New(s).Generate(context.Background(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "userPassword:") {
		t.Fatalf("generated LDIF contains userPassword:\n%s", string(data))
	}
}

func TestGeneratorUsesNumericStringSyntax(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(testSchema))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "generated-*.ldif")
	if err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	cfg := DefaultConfig()
	cfg.Count = 20
	cfg.OutputPath = out.Name()
	cfg.OptionalFillPercent = 100
	cfg.SelectedAttributes = map[string]bool{"x121address": true}
	cfg.Tree.GroupPercent = 0
	cfg.Tree.ComputerPercent = 0
	cfg.Tree.ServicePercent = 0
	cfg.Tree.PrivilegedPercent = 0
	policy := DefaultAttributeSupportPolicy().EnableSubstrs("numericStringSubstringsMatch")
	if _, err := NewWithAttributeSupportPolicy(s, policy).Generate(context.Background(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?m)^x121Address: ([0-9 ]+)$`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	if len(matches) == 0 {
		t.Fatalf("generated LDIF missing x121Address:\n%s", string(data))
	}
	for _, match := range matches {
		if strings.TrimSpace(match[1]) == "" {
			t.Fatalf("x121Address is blank numeric string: %q", match[0])
		}
	}
}

func TestGeneratorUsesPostalAddressSyntax(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(testSchema))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "generated-*.ldif")
	if err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	cfg := DefaultConfig()
	cfg.Count = 20
	cfg.OutputPath = out.Name()
	cfg.OptionalFillPercent = 100
	cfg.SelectedAttributes = map[string]bool{"homepostaladdress": true}
	cfg.Tree.GroupPercent = 0
	cfg.Tree.ComputerPercent = 0
	cfg.Tree.ServicePercent = 0
	cfg.Tree.PrivilegedPercent = 0
	policy := DefaultAttributeSupportPolicy().EnableSubstrs("caseIgnoreListSubstringsMatch")
	if _, err := NewWithAttributeSupportPolicy(s, policy).Generate(context.Background(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?m)^homePostalAddress: ([[:print:]]+)$`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	if len(matches) == 0 {
		t.Fatalf("generated LDIF missing homePostalAddress:\n%s", string(data))
	}
	for _, match := range matches {
		if strings.Count(match[1], "$") != 2 {
			t.Fatalf("homePostalAddress is not postal-address formatted: %q", match[0])
		}
	}
}

func TestDefaultValuesFollowLDAPSyntaxes(t *testing.T) {
	cases := []struct {
		name    string
		syntax  string
		pattern string
	}{
		{name: "attrType", syntax: "1.3.6.1.4.1.1466.115.121.1.3", pattern: `^\( 1\.3\.6\.1\.4\.1\.55555\.\d+ NAME 'generatedAttr\d+' SYNTAX 1\.3\.6\.1\.4\.1\.1466\.115\.121\.1\.15 \)$`},
		{name: "bit", syntax: "1.3.6.1.4.1.1466.115.121.1.6", pattern: `^'[01]{8}'B$`},
		{name: "bool", syntax: "1.3.6.1.4.1.1466.115.121.1.7", pattern: `^(TRUE|FALSE)$`},
		{name: "country", syntax: "1.3.6.1.4.1.1466.115.121.1.11", pattern: `^US$`},
		{name: "countryCode", syntax: "1.3.6.1.4.1.1466.115.121.1.27", pattern: `^\d+$`},
		{name: "dnValue", syntax: "1.3.6.1.4.1.1466.115.121.1.12", pattern: `^uid=user0000001,dc=example,dc=com$`},
		{name: "delivery", syntax: "1.3.6.1.4.1.1466.115.121.1.14", pattern: `^telephone \$ physical$`},
		{name: "dir", syntax: "1.3.6.1.4.1.1466.115.121.1.15", pattern: `^dir 1 \d{4}$`},
		{name: "ditContent", syntax: "1.3.6.1.4.1.1466.115.121.1.16", pattern: `^\( 1\.3\.6\.1\.4\.1\.55555\.\d+ NAME 'generatedContentRule\d+' AUX top MAY cn \)$`},
		{name: "ditStructure", syntax: "1.3.6.1.4.1.1466.115.121.1.17", pattern: `^\( 1 NAME 'generatedRule1' FORM 1\.3\.6\.1\.4\.1\.55555\.1 \)$`},
		{name: "fax", syntax: "1.3.6.1.4.1.1466.115.121.1.22", pattern: `^\+1 555 [0-9]{3} [0-9]{4}\$fineResolution$`},
		{name: "time", syntax: "1.3.6.1.4.1.1466.115.121.1.24", pattern: `^\d{14}Z$`},
		{name: "ia5", syntax: "1.3.6.1.4.1.1466.115.121.1.26", pattern: `^ia5-1-\d{4}$`},
		{name: "int", syntax: "1.3.6.1.4.1.1466.115.121.1.27", pattern: `^\d+$`},
		{name: "jpeg", syntax: "1.3.6.1.4.1.1466.115.121.1.28", pattern: `^#ffd8[0-9a-f]+ffd9$`},
		{name: "matchRule", syntax: "1.3.6.1.4.1.1466.115.121.1.30", pattern: `^\( 1\.3\.6\.1\.4\.1\.55555\.\d+ NAME 'generatedMatch\d+' SYNTAX 1\.3\.6\.1\.4\.1\.1466\.115\.121\.1\.15 \)$`},
		{name: "matchUse", syntax: "1.3.6.1.4.1.1466.115.121.1.31", pattern: `^\( 1\.3\.6\.1\.4\.1\.55555\.\d+ NAME 'generatedMatchUse\d+' APPLIES cn \)$`},
		{name: "nameUID", syntax: "1.3.6.1.4.1.1466.115.121.1.34", pattern: `^uid=user0000001,dc=example,dc=com#'[01]{8}'B$`},
		{name: "nameForm", syntax: "1.3.6.1.4.1.1466.115.121.1.35", pattern: `^\( 1\.3\.6\.1\.4\.1\.55555\.\d+ NAME 'generatedNameForm\d+' OC top MUST cn \)$`},
		{name: "numeric", syntax: "1.3.6.1.4.1.1466.115.121.1.36", pattern: `^[0-9 ]+$`},
		{name: "objectClass", syntax: "1.3.6.1.4.1.1466.115.121.1.37", pattern: `^\( 1\.3\.6\.1\.4\.1\.55555\.\d+ NAME 'generatedClass\d+' SUP top STRUCTURAL MUST cn \)$`},
		{name: "oid", syntax: "1.3.6.1.4.1.1466.115.121.1.38", pattern: `^1\.3\.6\.1\.4\.1\.55555\.\d+$`},
		{name: "octets", syntax: "1.3.6.1.4.1.1466.115.121.1.40", pattern: `^\{base64\}[A-Za-z0-9+/]+=*$`},
		{name: "postal", syntax: "1.3.6.1.4.1.1466.115.121.1.41", pattern: `^[^$]+\$[^$]+\$USA$`},
		{name: "printable", syntax: "1.3.6.1.4.1.1466.115.121.1.44", pattern: `^printable-1-\d{4}$`},
		{name: "phone", syntax: "1.3.6.1.4.1.1466.115.121.1.50", pattern: `^\+1 555 [0-9]{3} [0-9]{4}$`},
		{name: "teletex", syntax: "1.3.6.1.4.1.1466.115.121.1.51", pattern: `^terminal1\$graphic:ascii$`},
		{name: "telex", syntax: "1.3.6.1.4.1.1466.115.121.1.52", pattern: `^\d{6}\$US\$ANSWER$`},
		{name: "utc", syntax: "1.3.6.1.4.1.1466.115.121.1.53", pattern: `^\d{12}Z$`},
		{name: "ldapSyntax", syntax: "1.3.6.1.4.1.1466.115.121.1.54", pattern: `^\( 1\.3\.6\.1\.4\.1\.55555\.\d+ DESC 'Generated syntax' \)$`},
		{name: "uuid", syntax: "1.3.6.1.1.16.1", pattern: `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entry := EntryContext{
				Index:  0,
				Type:   EntryTypeUser,
				DN:     "uid=user0000001,dc=example,dc=com",
				UID:    "user0000001",
				Rand:   rand.New(rand.NewSource(1)),
				BaseDN: "dc=example,dc=com",
			}
			values := defaultValue(schema.AttributeType{Names: []string{tc.name}, Syntax: tc.syntax}, entry)
			if len(values) != 1 {
				t.Fatalf("values = %#v, want one value", values)
			}
			if !regexp.MustCompile(tc.pattern).MatchString(values[0]) {
				t.Fatalf("value for syntax %s = %q, want pattern %s", tc.syntax, values[0], tc.pattern)
			}
		})
	}
}

func TestGeneratorSkipsUnsupportedAttributeConstructions(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(testSchema))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "generated-*.ldif")
	if err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	cfg := DefaultConfig()
	cfg.Count = 20
	cfg.OutputPath = out.Name()
	cfg.OptionalFillPercent = 100
	cfg.SelectedAttributes = map[string]bool{
		"authpassword":            true,
		"homepostaladdress":       true,
		"internationalisdnnumber": true,
		"testattroid":             true,
		"telephonenumber":         true,
		"usersmimecertificate":    true,
		"x121address":             true,
	}
	cfg.Tree.GroupPercent = 0
	cfg.Tree.ComputerPercent = 0
	cfg.Tree.ServicePercent = 0
	cfg.Tree.PrivilegedPercent = 0
	if _, err := New(s).Generate(context.Background(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, attr := range []string{"authPassword", "homePostalAddress", "internationalISDNNumber", "testAttrOID", "telephoneNumber", "userSMIMECertificate", "x121Address"} {
		if strings.Contains(text, "\n"+attr+":") || strings.Contains(text, "\n"+attr+"::") {
			t.Fatalf("generated LDIF contains disabled attribute %s:\n%s", attr, text)
		}
	}
}

func TestGeneratorCanEnableUnsupportedConstructionsInCode(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(testSchema))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "generated-*.ldif")
	if err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	cfg := DefaultConfig()
	cfg.Count = 20
	cfg.OutputPath = out.Name()
	cfg.OptionalFillPercent = 100
	cfg.SelectedAttributes = map[string]bool{"usersmimecertificate": true}
	cfg.Tree.GroupPercent = 0
	cfg.Tree.ComputerPercent = 0
	cfg.Tree.ServicePercent = 0
	cfg.Tree.PrivilegedPercent = 0
	policy := DefaultAttributeSupportPolicy().EnableSyntaxes("1.3.6.1.4.1.1466.115.121.1.5")
	if _, err := NewWithAttributeSupportPolicy(s, policy).Generate(context.Background(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\nuserSMIMECertificate:: ") {
		t.Fatalf("generated LDIF missing enabled userSMIMECertificate:\n%s", string(data))
	}
}

func TestGeneratorUsesInheritedDNSyntax(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(testSchema))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "generated-*.ldif")
	if err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	cfg := DefaultConfig()
	cfg.Count = 20
	cfg.OutputPath = out.Name()
	cfg.OptionalFillPercent = 100
	cfg.SelectedAttributes = map[string]bool{"seealso": true}
	cfg.Tree.GroupPercent = 0
	cfg.Tree.ComputerPercent = 0
	cfg.Tree.ServicePercent = 0
	cfg.Tree.PrivilegedPercent = 0
	if _, err := New(s).Generate(context.Background(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`(?m)^seeAlso: uid=user\d{7},ou=Users,dc=example,dc=com$`)
	if !re.Match(data) {
		t.Fatalf("generated LDIF missing DN-valued seeAlso:\n%s", string(data))
	}
}

func TestGeneratorAlwaysWritesMustAttributes(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(testSchema))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "generated-*.ldif")
	if err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	cfg := DefaultConfig()
	cfg.Count = 10
	cfg.OutputPath = out.Name()
	cfg.OptionalFillPercent = 0
	cfg.SelectedAttributes = map[string]bool{"description": true}
	cfg.ObjectClasses[EntryTypeUser] = []string{"mustRichUser"}
	cfg.Tree.GroupPercent = 0
	cfg.Tree.ComputerPercent = 0
	cfg.Tree.ServicePercent = 0
	cfg.Tree.PrivilegedPercent = 0
	if _, err := New(s).Generate(context.Background(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, attr := range []string{"cn", "sn", "uid", "mail", "seeAlso"} {
		if !strings.Contains(text, "\n"+attr+": ") {
			t.Fatalf("generated LDIF missing MUST attribute %s:\n%s", attr, text)
		}
	}
}

func TestGeneratorErrorsWhenMustAttributeCannotBeGenerated(t *testing.T) {
	s, err := schema.Parse(strings.NewReader(testSchema))
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "generated-*.ldif")
	if err != nil {
		t.Fatal(err)
	}
	_ = out.Close()
	cfg := DefaultConfig()
	cfg.Count = 10
	cfg.OutputPath = out.Name()
	cfg.ObjectClasses[EntryTypeUser] = []string{"mustUnsupportedUser"}
	cfg.Tree.GroupPercent = 0
	cfg.Tree.ComputerPercent = 0
	cfg.Tree.ServicePercent = 0
	cfg.Tree.PrivilegedPercent = 0
	_, err = New(s).Generate(context.Background(), cfg, nil)
	if err == nil || !strings.Contains(err.Error(), `required attribute "telephoneNumber" is disabled`) {
		t.Fatalf("Generate error = %v, want required disabled attribute error", err)
	}
}

func TestAttributeSupportPolicyCanToggleConstructions(t *testing.T) {
	attr := schema.AttributeType{
		Names:  []string{"userSMIMECertificate"},
		Syntax: "1.3.6.1.4.1.1466.115.121.1.5",
	}
	policy := DefaultAttributeSupportPolicy()
	if policy.Allows(attr) {
		t.Fatal("default policy should disable unsupported binary syntax")
	}
	policy = policy.EnableSyntaxes("1.3.6.1.4.1.1466.115.121.1.5")
	if !policy.Allows(attr) {
		t.Fatal("enabled syntax should be allowed")
	}
	policy = policy.DisableSyntaxes("1.3.6.1.4.1.1466.115.121.1.5")
	if policy.Allows(attr) {
		t.Fatal("disabled syntax should be rejected again")
	}
}

const testSchema = `
attributeTypes: ( 2.5.4.0 NAME 'objectClass' )
attributeTypes: ( 2.5.4.3 NAME 'cn' )
attributeTypes: ( 2.5.4.4 NAME 'sn' )
attributeTypes: ( 0.9.2342.19200300.100.1.1 NAME 'uid' )
attributeTypes: ( 0.9.2342.19200300.100.1.3 NAME 'mail' )
attributeTypes: ( 2.5.4.41 NAME 'name' NO-USER-MODIFICATION )
attributeTypes: ( 2.5.4.42 NAME 'givenName' )
attributeTypes: ( 2.5.4.13 NAME 'description' )
attributeTypes: ( 2.5.4.11 NAME 'ou' )
attributeTypes: ( 2.5.4.31 NAME 'member' )
attributeTypes: ( 2.5.4.35 NAME 'userPassword' )
attributeTypes: ( 2.5.4.49 NAME 'distinguishedName' SYNTAX 1.3.6.1.4.1.1466.115.121.1.12 )
attributeTypes: ( 2.5.4.34 NAME 'seeAlso' SUP distinguishedName )
attributeTypes: ( 2.5.4.20 NAME 'telephoneNumber' SUBSTR telephoneNumberSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.50 )
attributeTypes: ( 2.5.4.24 NAME 'x121Address' EQUALITY numericStringMatch SUBSTR numericStringSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.36 )
attributeTypes: ( 2.5.4.25 NAME 'internationalISDNNumber' SUBSTR numericStringSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.36 )
attributeTypes: ( 0.9.2342.19200300.100.1.39 NAME 'homePostalAddress' SUBSTR caseIgnoreListSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.41 )
attributeTypes: ( 1.3.6.1.4.1.99999.1.1.1 NAME 'testAttrOID' ORDERING objectIdentifierOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.38 )
attributeTypes: ( 2.16.840.1.113730.3.1.40 NAME 'userSMIMECertificate' SYNTAX 1.3.6.1.4.1.1466.115.121.1.5 )
attributeTypes: ( 1.3.6.1.4.1.4203.1.3.4 NAME 'authPassword' EQUALITY 1.3.6.1.4.1.4203.1.2.2 SYNTAX 1.3.6.1.4.1.4203.1.1.2 )
attributeTypes: ( 1.2.840.113556.1.2.102 NAME 'memberOf' NO-USER-MODIFICATION )
attributeTypes: ( 0.9.2342.19200300.100.1.10 NAME 'manager' SINGLE-VALUE )
objectClasses: ( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )
objectClasses: ( 2.5.6.5 NAME 'organizationalUnit' SUP top STRUCTURAL MUST ou )
objectClasses: ( 2.16.840.1.113730.3.2.2 NAME 'inetOrgPerson' SUP top STRUCTURAL MUST ( cn $ sn ) MAY ( uid $ mail $ givenName $ description $ memberOf $ manager $ userPassword $ seeAlso $ telephoneNumber $ x121Address $ internationalISDNNumber $ homePostalAddress $ testAttrOID $ userSMIMECertificate $ authPassword $ name ) )
objectClasses: ( 1.2.3.10 NAME 'mustRichUser' SUP top STRUCTURAL MUST ( cn $ sn $ uid $ mail $ seeAlso ) MAY description )
objectClasses: ( 1.2.3.11 NAME 'mustUnsupportedUser' SUP top STRUCTURAL MUST ( cn $ sn $ telephoneNumber ) )
objectClasses: ( 1.2.643.4.38.2.2.1 NAME 'privUser' SUP inetOrgPerson STRUCTURAL MAY description )
objectClasses: ( 2.5.6.9 NAME 'groupOfNames' SUP top STRUCTURAL MUST cn MAY ( member $ description ) )
objectClasses: ( 2.5.6.14 NAME 'device' SUP top STRUCTURAL MUST cn MAY description )
objectClasses: ( 0.9.2342.19200300.100.4.5 NAME 'account' SUP top STRUCTURAL MUST uid MAY description )
objectClasses: ( 1.2.643.4.38.2.2.2 NAME 'serviceUser' SUP inetOrgPerson STRUCTURAL MAY description )
`

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
