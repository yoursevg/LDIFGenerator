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
		return []string{fmt.Sprintf("+1 555 %03d %04d", e.Rand.Intn(900)+100, e.Rand.Intn(10000))}, nil
	}))
	r.Register("description", generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		return []string{fmt.Sprintf("Generated %s entry %d for LDAP load testing", e.Type, e.Index+1)}, nil
	}))
	r.Register("userPassword", generatorFunc(func(_ context.Context, _ schema.AttributeType, e EntryContext) ([]string, error) {
		return []string{fmt.Sprintf("{SSHA}fake%08x", e.Rand.Uint32())}, nil
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
	syntax := strings.ToLower(attr.Syntax)
	switch {
	case strings.Contains(name, "boolean"):
		if e.Rand.Intn(2) == 0 {
			return []string{"FALSE"}
		}
		return []string{"TRUE"}
	case strings.Contains(name, "number") || strings.Contains(name, "count") || strings.Contains(syntax, "integer"):
		return []string{fmt.Sprintf("%d", e.Rand.Intn(1000000))}
	case strings.Contains(name, "dn"):
		return []string{e.DN}
	default:
		return []string{fmt.Sprintf("%s-%d-%04d", attr.PrimaryName(), e.Index+1, e.Rand.Intn(10000))}
	}
}

var fakeGivenNames = []string{"Alex", "Jordan", "Taylor", "Morgan", "Casey", "Riley", "Jamie", "Avery"}
var fakeSurnames = []string{"Smith", "Johnson", "Brown", "Davis", "Miller", "Wilson", "Moore", "Taylor"}
