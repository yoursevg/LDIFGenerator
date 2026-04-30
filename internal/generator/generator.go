package generator

import (
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yoursevg/LDIFGenerator/internal/ldif"
	"github.com/yoursevg/LDIFGenerator/internal/schema"
	"github.com/yoursevg/LDIFGenerator/internal/validation"
)

type ProgressFunc func(written, total int)

type EntryContext struct {
	Index     int
	Type      EntryType
	DN        string
	BaseDN    string
	CN        string
	UID       string
	GivenName string
	SN        string
	Rand      *rand.Rand
	Related   Relationships
}

type AttributeGenerator interface {
	Generate(ctx context.Context, attr schema.AttributeType, entry EntryContext) ([]string, error)
}

type Generator struct {
	schema    *schema.Schema
	fakes     *FakeRegistry
	validator *validation.Validator
}

func New(s *schema.Schema) *Generator {
	return &Generator{
		schema:    s,
		fakes:     NewFakeRegistry(),
		validator: validation.New(s),
	}
}

func (g *Generator) Generate(ctx context.Context, cfg GeneratorConfig, progress ProgressFunc) (Report, error) {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 1000
	}
	if cfg.OptionalFillPercent < 0 {
		cfg.OptionalFillPercent = 0
	}
	if cfg.OptionalFillPercent > 100 {
		cfg.OptionalFillPercent = 100
	}
	if strings.TrimSpace(cfg.BaseDN) == "" {
		return Report{}, fmt.Errorf("base DN is required")
	}
	if cfg.Count <= 0 {
		return Report{}, fmt.Errorf("count must be positive")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0o755); err != nil && filepath.Dir(cfg.OutputPath) != "." {
		return Report{}, err
	}
	start := time.Now()
	file, err := os.Create(cfg.OutputPath)
	if err != nil {
		return Report{}, err
	}
	defer file.Close()

	writer := ldif.NewWriter(file, 2*1024*1024)
	rng := rand.New(rand.NewSource(cfg.Seed))
	plan := BuildPlan(cfg, rng)
	rel := BuildRelationships(cfg, plan, rng)
	report := Report{StartedAt: start, OutputPath: cfg.OutputPath}

	if cfg.Tree.Mode != TreeModeFlat {
		for _, rec := range g.ouRecords(cfg) {
			if err := writer.WriteRecord(rec); err != nil {
				return report, err
			}
			report.Records++
		}
	}
	for i, typ := range plan {
		select {
		case <-ctx.Done():
			_ = writer.Flush()
			return report, ctx.Err()
		default:
		}
		rec, err := g.buildRecord(ctx, cfg, typ, i, rel, rng)
		if err != nil {
			return report, err
		}
		if err := g.validator.ValidateRecord(rec, cfg.ObjectClasses[typ], cfg.StrictMode); err != nil {
			return report, err
		}
		if err := writer.WriteRecord(rec); err != nil {
			return report, err
		}
		report.Records++
		if progress != nil && (i+1)%cfg.BatchSize == 0 {
			progress(i+1, cfg.Count)
		}
	}
	if progress != nil {
		progress(cfg.Count, cfg.Count)
	}
	if err := writer.Flush(); err != nil {
		return report, err
	}
	info, err := file.Stat()
	if err == nil {
		report.FileBytes = info.Size()
	}
	report.FinishedAt = time.Now()
	report.Duration = report.FinishedAt.Sub(report.StartedAt)
	if report.Duration > 0 {
		report.RecordsPerSec = float64(report.Records) / report.Duration.Seconds()
	}
	return report, nil
}

func (g *Generator) ouRecords(cfg GeneratorConfig) []ldif.Record {
	seen := map[string]bool{}
	var out []ldif.Record
	for _, ou := range []string{cfg.Tree.UserOU, cfg.Tree.PrivilegedOU, cfg.Tree.GroupOU, cfg.Tree.ComputerOU, cfg.Tree.ServiceOU} {
		if ou == "" || seen[strings.ToLower(ou)] {
			continue
		}
		seen[strings.ToLower(ou)] = true
		rec := ldif.NewRecord(fmt.Sprintf("ou=%s,%s", escapeDNValue(ou), cfg.BaseDN))
		rec.Add("objectClass", "top", "organizationalUnit")
		rec.Add("ou", ou)
		out = append(out, rec)
	}
	return out
}

func (g *Generator) buildRecord(ctx context.Context, cfg GeneratorConfig, typ EntryType, index int, rel Relationships, rng *rand.Rand) (ldif.Record, error) {
	ec := baseEntryContext(cfg, typ, index, rel, rng)
	rec := ldif.NewRecord(ec.DN)
	classes := cfg.ObjectClasses[typ]
	if len(classes) == 0 {
		return rec, fmt.Errorf("objectClass not configured for %s", typ)
	}
	resolved, err := g.schema.ResolveObjectClasses(classes)
	if err != nil {
		return rec, err
	}
	for _, oc := range resolved.ObjectClasses {
		rec.Add("objectClass", oc.PrimaryName())
	}
	attrNames := mergeAttributes(resolved.Must, chooseMay(resolved.May, cfg, rng))
	for _, name := range attrNames {
		if schema.NormalizeName(name) == "objectclass" {
			continue
		}
		attr, ok := g.schema.Attribute(name)
		if !ok {
			if cfg.StrictMode {
				return rec, fmt.Errorf("attribute %q is not defined in schema", name)
			}
			attr = schema.AttributeType{Names: []string{name}}
		}
		values, err := g.fakes.Generate(ctx, attr, ec)
		if err != nil {
			return rec, err
		}
		if len(values) > 0 {
			addGeneratedValues(&rec, attr.PrimaryName(), values)
		}
	}
	for _, extra := range rel.ExtraAttributes[ec.DN] {
		attr, ok := g.schema.Attribute(extra.Name)
		if ok && recordHasAttribute(rec, attr) {
			continue
		}
		values := extra.Values
		if ok && attr.SingleValue && len(values) > 1 {
			values = values[:1]
		}
		rec.Add(extra.Name, values...)
	}
	return rec, nil
}

func addGeneratedValues(rec *ldif.Record, name string, values []string) {
	var text []string
	for _, value := range values {
		if raw, ok := strings.CutPrefix(value, "{base64}"); ok {
			decoded, err := base64.StdEncoding.DecodeString(raw)
			if err == nil {
				rec.AddBinary(name, decoded)
				continue
			}
		}
		text = append(text, value)
	}
	rec.Add(name, text...)
}

func recordHasAttribute(rec ldif.Record, attr schema.AttributeType) bool {
	keys := map[string]bool{}
	for _, name := range attr.Names {
		keys[schema.NormalizeName(name)] = true
	}
	if attr.OID != "" {
		keys[schema.NormalizeName(attr.OID)] = true
	}
	for _, existing := range rec.Attributes {
		if keys[schema.NormalizeName(existing.Name)] {
			return true
		}
	}
	return false
}

func mergeAttributes(groups ...[]string) []string {
	seen := map[string]bool{}
	var out []string
	for _, group := range groups {
		for _, name := range group {
			key := schema.NormalizeName(name)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, name)
		}
	}
	return out
}

func chooseMay(may []string, cfg GeneratorConfig, rng *rand.Rand) []string {
	var out []string
	for _, name := range may {
		if len(cfg.SelectedAttributes) > 0 && !cfg.SelectedAttributes[schema.NormalizeName(name)] {
			continue
		}
		if rng.Intn(100) < cfg.OptionalFillPercent {
			out = append(out, name)
		}
	}
	return out
}

func baseEntryContext(cfg GeneratorConfig, typ EntryType, index int, rel Relationships, rng *rand.Rand) EntryContext {
	uid := fmt.Sprintf("%s%07d", typePrefix(typ), index+1)
	given := fakeGivenNames[index%len(fakeGivenNames)]
	sn := fakeSurnames[(index/len(fakeGivenNames))%len(fakeSurnames)]
	cn := given + " " + sn
	if typ == EntryTypeGroup {
		cn = fmt.Sprintf("group-%07d", index+1)
		uid = cn
	}
	if typ == EntryTypePrivileged {
		uid = fmt.Sprintf("priv%07d", index+1)
	}
	if typ == EntryTypeComputer {
		cn = fmt.Sprintf("pc-%07d", index+1)
		uid = cn + "$"
	}
	if typ == EntryTypeService {
		cn = fmt.Sprintf("svc-%07d", index+1)
		uid = cn
	}
	parent := cfg.BaseDN
	if cfg.Tree.Mode != TreeModeFlat {
		parent = fmt.Sprintf("ou=%s,%s", escapeDNValue(ouForType(cfg, typ)), cfg.BaseDN)
	}
	dn := fmt.Sprintf("uid=%s,%s", escapeDNValue(uid), parent)
	if typ == EntryTypeGroup {
		dn = fmt.Sprintf("cn=%s,%s", escapeDNValue(cn), parent)
	}
	if typ == EntryTypeComputer {
		dn = fmt.Sprintf("cn=%s,%s", escapeDNValue(cn), parent)
	}
	return EntryContext{Index: index, Type: typ, DN: dn, BaseDN: cfg.BaseDN, CN: cn, UID: uid, GivenName: given, SN: sn, Rand: rng, Related: rel}
}

func ouForType(cfg GeneratorConfig, typ EntryType) string {
	switch typ {
	case EntryTypeGroup:
		return cfg.Tree.GroupOU
	case EntryTypePrivileged:
		return cfg.Tree.PrivilegedOU
	case EntryTypeComputer:
		return cfg.Tree.ComputerOU
	case EntryTypeService:
		return cfg.Tree.ServiceOU
	default:
		return cfg.Tree.UserOU
	}
}

func typePrefix(typ EntryType) string {
	switch typ {
	case EntryTypePrivileged:
		return "priv"
	case EntryTypeComputer:
		return "pc"
	case EntryTypeService:
		return "svc"
	default:
		return "user"
	}
}

func escapeDNValue(v string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `,`, `\,`, `+`, `\+`, `"`, `\"`, `<`, `\<`, `>`, `\>`, `;`, `\;`)
	return replacer.Replace(v)
}
