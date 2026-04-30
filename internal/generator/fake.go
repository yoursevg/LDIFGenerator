package generator

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/yoursevg/LDIFGenerator/internal/schema"
)

type generatorFunc func(context.Context, schema.AttributeType, EntryContext) ([]string, error)

func (f generatorFunc) Generate(ctx context.Context, attr schema.AttributeType, entry EntryContext) ([]string, error) {
	return f(ctx, attr, entry)
}

type FakeRegistry struct {
	byName map[string]AttributeGenerator
}

func NewFakeRegistry() *FakeRegistry {
	r := &FakeRegistry{byName: map[string]AttributeGenerator{}}
	r.Register("cn", generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		return []string{e.CN}, nil
	}))
	r.Register("uid", generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		return []string{e.UID}, nil
	}))
	r.Register("sn", generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		return []string{e.SN}, nil
	}))
	r.Register("givenName", generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		return []string{e.GivenName}, nil
	}))
	r.Register("displayName", generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		return []string{e.CN}, nil
	}))
	r.Register("mail", generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		return []string{strings.ToLower(e.UID) + "@example.com"}, nil
	}))
	r.Register("telephoneNumber", generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		return []string{telephoneNumber(e)}, nil
	}))
	r.Register("description", generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		return []string{fmt.Sprintf("Generated %s entry %d for LDAP load testing", e.Type, e.Index+1)}, nil
	}))
	r.Register("entryUUID", uuidGenerator())
	r.Register("objectGUID", binaryIDGenerator())
	r.Register("objectSid", binaryIDGenerator())
	r.Register("createTimestamp", timestampGenerator())
	r.Register("modifyTimestamp", timestampGenerator())
	r.Register("manager", generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		if values := e.Related.ExtraAttributes[e.DN]; len(values) > 0 {
			for _, v := range values {
				if strings.EqualFold(v.Name, "manager") {
					return v.Values, nil
				}
			}
		}
		return nil, nil
	}))
	r.Register("member", generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		return e.Related.GroupMembers[e.DN], nil
	}))
	r.Register("memberOf", generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		return e.Related.UserGroups[e.DN], nil
	}))
	return r
}

func (r *FakeRegistry) Register(name string, g AttributeGenerator) {
	r.byName[schema.NormalizeName(name)] = g
}

func (r *FakeRegistry) Generate(ctx context.Context, attr schema.AttributeType, entry EntryContext) ([]string, error) {
	for _, name := range attr.Names {
		if g, ok := r.byName[schema.NormalizeName(name)]; ok {
			return g.Generate(ctx, attr, entry)
		}
	}
	return defaultValue(attr, entry), nil
}

func uuidGenerator() AttributeGenerator {
	return generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		return []string{fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", e.Rand.Uint32(), e.Rand.Uint32()&0xffff, e.Rand.Uint32()&0xffff, e.Rand.Uint32()&0xffff, e.Rand.Uint64()&0xffffffffffff)}, nil
	})
}

func binaryIDGenerator() AttributeGenerator {
	return generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		buf := make([]byte, 16)
		_, _ = e.Rand.Read(buf)
		return []string{"{base64}" + base64.StdEncoding.EncodeToString(buf)}, nil
	})
}

func timestampGenerator() AttributeGenerator {
	return generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		t := time.Unix(1704067200+int64(e.Index*17), 0).UTC()
		return []string{t.Format("20060102150405Z")}, nil
	})
}

func defaultValue(attr schema.AttributeType, e EntryContext) []string {
	name := strings.ToLower(attr.PrimaryName())
	if value, ok := valueByName(name, e); ok {
		return []string{value}
	}
	if value, ok := valueBySyntax(attr, e); ok {
		return []string{value}
	}
	switch {
	case strings.Contains(name, "boolean"):
		if e.Rand.Intn(2) == 0 {
			return []string{"FALSE"}
		}
		return []string{"TRUE"}
	case strings.Contains(name, "number") || strings.Contains(name, "count"):
		return []string{fmt.Sprintf("%d", e.Rand.Intn(1000000))}
	case strings.Contains(name, "dn"):
		return []string{e.DN}
	default:
		return []string{directoryString(attr, e)}
	}
}

func valueByName(name string, e EntryContext) (string, bool) {
	switch {
	case strings.Contains(name, "postaladdress") || name == "postaladdress" || name == "registeredaddress":
		return postalAddress(e), true
	case strings.Contains(name, "streetaddress") || name == "street":
		return streetAddress(e), true
	case strings.Contains(name, "homeaddress"):
		return streetAddress(e), true
	case name == "postalcode" || name == "zipcode":
		return fmt.Sprintf("%05d", 10000+e.Rand.Intn(90000)), true
	case name == "c" || strings.Contains(name, "country"):
		return "US", true
	case name == "l" || name == "localityname" || strings.Contains(name, "city"):
		return cityName(e), true
	case name == "st" || strings.Contains(name, "state"):
		return "CA", true
	case strings.Contains(name, "homedirectory") || strings.Contains(name, "unixhomedirectory"):
		return "/home/" + strings.ToLower(e.UID), true
	case strings.Contains(name, "macaddress"):
		return macAddress(e), true
	case strings.Contains(name, "proxyaddresses"):
		return "SMTP:" + strings.ToLower(e.UID) + "@example.com", true
	case strings.Contains(name, "email") || strings.Contains(name, "mail"):
		return strings.ToLower(e.UID) + "@example.com", true
	case strings.Contains(name, "phone") || strings.Contains(name, "telephone") || strings.Contains(name, "mobile"):
		return telephoneNumber(e), true
	}
	return "", false
}

func valueBySyntax(attr schema.AttributeType, e EntryContext) (string, bool) {
	switch syntaxOID(attr.Syntax) {
	case "1.3.6.1.4.1.1466.115.121.1.7":
		if e.Rand.Intn(2) == 0 {
			return "FALSE", true
		}
		return "TRUE", true
	case "1.3.6.1.4.1.1466.115.121.1.12", "2.5.5.1":
		return e.DN, true
	case "1.3.6.1.4.1.1466.115.121.1.24":
		return generalizedTime(e), true
	case "1.3.6.1.4.1.1466.115.121.1.27", "2.5.5.9", "2.5.5.16":
		return fmt.Sprintf("%d", e.Rand.Intn(1000000)), true
	case "1.3.6.1.4.1.1466.115.121.1.36":
		return numericString(e), true
	case "1.3.6.1.4.1.1466.115.121.1.41":
		return postalAddress(e), true
	case "1.3.6.1.4.1.1466.115.121.1.50":
		return telephoneNumber(e), true
	case "1.3.6.1.4.1.1466.115.121.1.53":
		return utcTime(e), true
	case "1.3.6.1.4.1.1466.115.121.1.11":
		return "US", true
	case "1.3.6.1.4.1.1466.115.121.1.6":
		return "'10101010'B", true
	case "1.3.6.1.4.1.1466.115.121.1.38":
		return fmt.Sprintf("1.3.6.1.4.1.55555.%d", e.Index+1), true
	case "1.3.6.1.4.1.1466.115.121.1.40", "1.3.6.1.4.1.1466.115.121.1.4", "1.3.6.1.4.1.1466.115.121.1.5", "1.3.6.1.4.1.1466.115.121.1.8", "1.3.6.1.4.1.1466.115.121.1.9", "1.3.6.1.4.1.1466.115.121.1.10", "1.3.6.1.4.1.1466.115.121.1.28":
		return binaryValue(e, 16), true
	case "1.3.6.1.4.1.1466.115.121.1.26", "1.3.6.1.4.1.1466.115.121.1.44", "2.5.5.5":
		return ia5String(attr, e), true
	case "1.3.6.1.4.1.1466.115.121.1.15", "2.5.5.12":
		return directoryString(attr, e), true
	}
	if isNumericString(attr) {
		return numericString(e), true
	}
	if strings.EqualFold(attr.Equality, "booleanMatch") {
		if e.Rand.Intn(2) == 0 {
			return "FALSE", true
		}
		return "TRUE", true
	}
	if strings.EqualFold(attr.Equality, "integerMatch") || strings.EqualFold(attr.Ordering, "integerOrderingMatch") {
		return fmt.Sprintf("%d", e.Rand.Intn(1000000)), true
	}
	return "", false
}

func syntaxOID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if i := strings.Index(raw, "{"); i >= 0 {
		raw = raw[:i]
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

func isNumericString(attr schema.AttributeType) bool {
	const numericStringSyntaxOID = "1.3.6.1.4.1.1466.115.121.1.36"
	syntax := syntaxOID(attr.Syntax)
	return strings.Contains(syntax, numericStringSyntaxOID) ||
		strings.Contains(syntax, "numeric string") ||
		strings.EqualFold(attr.Equality, "numericStringMatch") ||
		strings.EqualFold(attr.Substr, "numericStringSubstringsMatch")
}

func numericString(e EntryContext) string {
	return fmt.Sprintf("%03d %03d %04d", e.Rand.Intn(1000), e.Rand.Intn(1000), e.Rand.Intn(10000))
}

func telephoneNumber(e EntryContext) string {
	return fmt.Sprintf("+1 555 %03d %04d", e.Rand.Intn(900)+100, e.Rand.Intn(10000))
}

func generalizedTime(e EntryContext) string {
	t := time.Unix(1704067200+int64(e.Index*17), 0).UTC()
	return t.Format("20060102150405Z")
}

func utcTime(e EntryContext) string {
	t := time.Unix(1704067200+int64(e.Index*17), 0).UTC()
	return t.Format("060102150405Z")
}

func postalAddress(e EntryContext) string {
	return fmt.Sprintf("%d %s St$%s, CA %05d$USA", 100+e.Rand.Intn(9900), fakeSurnames[e.Index%len(fakeSurnames)], cityName(e), 10000+e.Rand.Intn(90000))
}

func streetAddress(e EntryContext) string {
	return fmt.Sprintf("%d %s St", 100+e.Rand.Intn(9900), fakeSurnames[e.Index%len(fakeSurnames)])
}

func cityName(e EntryContext) string {
	cities := []string{"Springfield", "Riverside", "Fairview", "Madison", "Georgetown", "Arlington"}
	return cities[e.Index%len(cities)]
}

func macAddress(e EntryContext) string {
	return fmt.Sprintf("02:%02x:%02x:%02x:%02x:%02x", e.Rand.Intn(256), e.Rand.Intn(256), e.Rand.Intn(256), e.Rand.Intn(256), e.Rand.Intn(256))
}

func binaryValue(e EntryContext, size int) string {
	buf := make([]byte, size)
	_, _ = e.Rand.Read(buf)
	return "{base64}" + base64.StdEncoding.EncodeToString(buf)
}

func ia5String(attr schema.AttributeType, e EntryContext) string {
	name := strings.ToLower(attr.PrimaryName())
	switch {
	case strings.Contains(name, "uri") || strings.Contains(name, "url"):
		return fmt.Sprintf("https://example.com/%s/%d", e.Type, e.Index+1)
	case strings.Contains(name, "path") || strings.Contains(name, "directory"):
		return "/var/lib/" + strings.ToLower(e.UID)
	default:
		return fmt.Sprintf("%s-%d-%04d", attr.PrimaryName(), e.Index+1, e.Rand.Intn(10000))
	}
}

func directoryString(attr schema.AttributeType, e EntryContext) string {
	return fmt.Sprintf("%s %d %04d", attr.PrimaryName(), e.Index+1, e.Rand.Intn(10000))
}

var fakeGivenNames = []string{"Alex", "Jordan", "Taylor", "Morgan", "Casey", "Riley", "Jamie", "Avery"}
var fakeSurnames = []string{"Smith", "Johnson", "Brown", "Davis", "Miller", "Wilson", "Moore", "Taylor"}
