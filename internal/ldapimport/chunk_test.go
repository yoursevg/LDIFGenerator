package ldapimport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitFilePhasesRecords(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.ldif")
	data := strings.Join([]string{
		"dn: ou=Users,dc=example,dc=com\nobjectClass: organizationalUnit\nou: Users\n\n",
		"dn: uid=user1,ou=Users,dc=example,dc=com\nobjectClass: inetOrgPerson\ncn: User One\nsn: One\n\n",
		"dn: cn=group1,ou=Groups,dc=example,dc=com\nobjectClass: groupOfNames\ncn: group1\nmember: uid=user1,ou=Users,dc=example,dc=com\n\n",
	}, "")
	if err := os.WriteFile(input, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	chunks, err := SplitFile(input, SplitOptions{WorkDir: filepath.Join(dir, "chunks"), ChunkRecords: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d, want 3", len(chunks))
	}
	want := []Phase{PhaseContainers, PhaseEntries, PhaseGroups}
	for i, chunk := range chunks {
		if chunk.Phase != want[i] {
			t.Fatalf("chunk %d phase = %s, want %s", i, PhaseName(chunk.Phase), PhaseName(want[i]))
		}
		if chunk.Records != 1 {
			t.Fatalf("chunk %d records = %d, want 1", i, chunk.Records)
		}
	}
}

func TestParseAttrsUnfoldsValues(t *testing.T) {
	attrs := parseAttrs("dn: cn=group,dc=example,dc=com\nobjectClass: group\nmember: uid=user,\n ou=Users,dc=example,dc=com\n\n")
	if got := attrs["member"][0]; got != "uid=user,ou=Users,dc=example,dc=com" {
		t.Fatalf("member = %q", got)
	}
}
