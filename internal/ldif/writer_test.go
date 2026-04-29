package ldif

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriterFormatsRecordAndBase64(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf, 0)
	rec := NewRecord("cn=user,dc=example,dc=com")
	rec.Add("objectClass", "inetOrgPerson")
	rec.Add("cn", "user")
	rec.Add("description", " leading")
	if err := w.WriteRecord(rec); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{
		"dn: cn=user,dc=example,dc=com\n",
		"objectClass: inetOrgPerson\n",
		"cn: user\n",
		"description:: IGxlYWRpbmc=\n\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestWriterFoldsLongLine(t *testing.T) {
	line := EncodeLine("description", Value{Text: strings.Repeat("a", 120)})
	if !strings.Contains(line, "\n ") {
		t.Fatalf("line was not folded: %q", line)
	}
}
