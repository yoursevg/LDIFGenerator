package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/yoursevg/LDIFGenerator/internal/schema"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: schemaaudit <schema.ldif>")
	}
	s, err := schema.ParseFiles([]string{os.Args[1]})
	if err != nil {
		log.Fatal(err)
	}
	attrs := uniqueAttrNames(s)
	ocs := uniqueOCNames(s)
	fmt.Printf("attributes=%d objectClasses=%d warnings=%d\n", len(attrs), len(ocs), len(s.Warnings))
	for _, warning := range s.Warnings {
		fmt.Println("warning:", warning)
	}
	fmt.Println("givenName loaded:", contains(attrs, "givenName"))
	if attr, ok := s.Attribute("givenName"); ok {
		fmt.Printf("givenName attr: oid=%s names=%v\n", attr.OID, attr.Names)
	} else {
		fmt.Println("givenName lookup: missing")
	}
	for _, name := range attrs {
		if strings.Contains(strings.ToLower(name), "given") || strings.Contains(strings.ToLower(name), "generation") {
			fmt.Println("near:", name)
		}
	}
}

func uniqueAttrNames(s *schema.Schema) []string {
	seen := map[string]bool{}
	var out []string
	for _, attr := range s.AttributeTypes {
		name := attr.PrimaryName()
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func uniqueOCNames(s *schema.Schema) []string {
	seen := map[string]bool{}
	var out []string
	for _, oc := range s.ObjectClasses {
		name := oc.PrimaryName()
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
