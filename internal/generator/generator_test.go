package generator

import (
	"context"
	"math/rand"
	"os"
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

const testSchema = `
attributeTypes: ( 2.5.4.0 NAME 'objectClass' )
attributeTypes: ( 2.5.4.3 NAME 'cn' )
attributeTypes: ( 2.5.4.4 NAME 'sn' )
attributeTypes: ( 0.9.2342.19200300.100.1.1 NAME 'uid' )
attributeTypes: ( 0.9.2342.19200300.100.1.3 NAME 'mail' )
attributeTypes: ( 2.5.4.42 NAME 'givenName' )
attributeTypes: ( 2.5.4.13 NAME 'description' )
attributeTypes: ( 2.5.4.11 NAME 'ou' )
attributeTypes: ( 2.5.4.31 NAME 'member' )
attributeTypes: ( 1.2.840.113556.1.2.102 NAME 'memberOf' )
attributeTypes: ( 0.9.2342.19200300.100.1.10 NAME 'manager' SINGLE-VALUE )
objectClasses: ( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )
objectClasses: ( 2.5.6.5 NAME 'organizationalUnit' SUP top STRUCTURAL MUST ou )
objectClasses: ( 2.16.840.1.113730.3.2.2 NAME 'inetOrgPerson' SUP top STRUCTURAL MUST ( cn $ sn ) MAY ( uid $ mail $ givenName $ description $ memberOf $ manager ) )
objectClasses: ( 1.2.643.4.38.2.2.1 NAME 'privUser' SUP inetOrgPerson STRUCTURAL MAY description )
objectClasses: ( 2.5.6.9 NAME 'groupOfNames' SUP top STRUCTURAL MUST ( cn $ member ) MAY description )
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
