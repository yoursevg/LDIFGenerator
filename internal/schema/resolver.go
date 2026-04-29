package schema

import "fmt"

type ResolvedObjectClass struct {
	ObjectClasses []ObjectClass `json:"objectClasses"`
	Must          []string      `json:"must"`
	May           []string      `json:"may"`
	Warnings      []string      `json:"warnings,omitempty"`
}

func (s *Schema) ResolveObjectClasses(names []string) (ResolvedObjectClass, error) {
	seenOC := map[string]bool{}
	seenMust := map[string]bool{}
	seenMay := map[string]bool{}
	var out ResolvedObjectClass
	for _, name := range names {
		if err := s.resolveObjectClass(name, seenOC, seenMust, seenMay, &out); err != nil {
			return out, err
		}
	}
	return out, nil
}

func (s *Schema) resolveObjectClass(name string, seenOC, seenMust, seenMay map[string]bool, out *ResolvedObjectClass) error {
	oc, ok := s.ObjectClass(name)
	if !ok {
		out.Warnings = append(out.Warnings, fmt.Sprintf("unknown objectClass %q", name))
		return nil
	}
	key := NormalizeName(oc.PrimaryName())
	if seenOC[key] {
		return nil
	}
	seenOC[key] = true
	for _, sup := range oc.SUP {
		if err := s.resolveObjectClass(sup, seenOC, seenMust, seenMay, out); err != nil {
			return err
		}
	}
	out.ObjectClasses = append(out.ObjectClasses, oc)
	for _, attr := range oc.Must {
		n := NormalizeName(attr)
		if !seenMust[n] {
			seenMust[n] = true
			out.Must = append(out.Must, attr)
		}
	}
	for _, attr := range oc.May {
		n := NormalizeName(attr)
		if !seenMay[n] {
			seenMay[n] = true
			out.May = append(out.May, attr)
		}
	}
	return nil
}
