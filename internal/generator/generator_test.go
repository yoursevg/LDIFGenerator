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
	if _, err := New(s).Generate(context.Background(), cfg, nil); err != nil {
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
	if _, err := New(s).Generate(context.Background(), cfg, nil); err != nil {
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
attributeTypes: ( 2.5.4.24 NAME 'x121Address' EQUALITY numericStringMatch SUBSTR numericStringSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.36 )
attributeTypes: ( 0.9.2342.19200300.100.1.39 NAME 'homePostalAddress' SYNTAX 1.3.6.1.4.1.1466.115.121.1.41 )
attributeTypes: ( 1.2.840.113556.1.2.102 NAME 'memberOf' NO-USER-MODIFICATION )
attributeTypes: ( 0.9.2342.19200300.100.1.10 NAME 'manager' SINGLE-VALUE )
objectClasses: ( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )
objectClasses: ( 2.5.6.5 NAME 'organizationalUnit' SUP top STRUCTURAL MUST ou )
objectClasses: ( 2.16.840.1.113730.3.2.2 NAME 'inetOrgPerson' SUP top STRUCTURAL MUST ( cn $ sn ) MAY ( uid $ mail $ givenName $ description $ memberOf $ manager $ userPassword $ x121Address $ homePostalAddress $ name ) )
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
