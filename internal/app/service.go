package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/yoursevg/LDIFGenerator/internal/generator"
	"github.com/yoursevg/LDIFGenerator/internal/schema"
)

type SchemaSummary struct {
	AttributeTypes []schema.AttributeType `json:"attributeTypes"`
	ObjectClasses  []ObjectClassSummary   `json:"objectClasses"`
	Warnings       []string               `json:"warnings,omitempty"`
}

type ObjectClassSummary struct {
	Name     string                 `json:"name"`
	OID      string                 `json:"oid"`
	Kind     schema.ObjectClassKind `json:"kind"`
	SUP      []string               `json:"sup,omitempty"`
	Must     []string               `json:"must"`
	May      []string               `json:"may"`
	Warnings []string               `json:"warnings,omitempty"`
}

type Progress struct {
	Written int               `json:"written"`
	Total   int               `json:"total"`
	Running bool              `json:"running"`
	Error   string            `json:"error,omitempty"`
	Report  *generator.Report `json:"report,omitempty"`
}

type Service struct {
	mu       sync.Mutex
	schema   *schema.Schema
	progress Progress
	cancel   context.CancelFunc
	report   generator.Report
	running  bool
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) LoadSchema(paths []string) (SchemaSummary, error) {
	resolvedPaths, err := schema.ResolveInputPaths(paths)
	if err != nil {
		return SchemaSummary{}, err
	}
	parsed, err := schema.ParseFiles(resolvedPaths)
	if err != nil {
		return SchemaSummary{}, err
	}
	s.mu.Lock()
	s.schema = parsed
	s.mu.Unlock()
	return summarize(parsed), nil
}

func (s *Service) LoadConfig(path string) (generator.GeneratorConfig, error) {
	var cfg generator.GeneratorConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (s *Service) SaveConfig(path string, cfg generator.GeneratorConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func (s *Service) StartGenerate(cfg generator.GeneratorConfig) (Progress, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return Progress{}, fmt.Errorf("generation is already running")
	}
	if s.schema == nil {
		s.mu.Unlock()
		return Progress{}, fmt.Errorf("schema is not loaded")
	}
	if cfg.OutputPath == "" {
		s.mu.Unlock()
		return Progress{}, fmt.Errorf("output LDIF path is required")
	}
	genSchema := s.schema
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true
	s.report = generator.Report{}
	s.progress = Progress{Total: cfg.Count, Running: true}
	progress := s.progress
	s.mu.Unlock()

	go func() {
		report, err := generator.New(genSchema).Generate(ctx, cfg, func(written, total int) {
			s.mu.Lock()
			s.progress.Written = written
			s.progress.Total = total
			s.progress.Running = true
			s.mu.Unlock()
		})
		s.mu.Lock()
		defer s.mu.Unlock()
		s.running = false
		s.cancel = nil
		s.report = report
		s.progress.Running = false
		s.progress.Total = cfg.Count
		if err != nil {
			s.progress.Error = err.Error()
			return
		}
		s.progress.Written = cfg.Count
		s.progress.Report = &report
	}()

	return progress, nil
}

func (s *Service) CancelGeneration() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *Service) Progress() Progress {
	s.mu.Lock()
	defer s.mu.Unlock()
	progress := s.progress
	if !s.running && s.report.OutputPath != "" {
		report := s.report
		progress.Report = &report
	}
	progress.Running = s.running
	return progress
}

func DefaultConfig() generator.GeneratorConfig {
	return generator.DefaultConfig()
}

func summarize(s *schema.Schema) SchemaSummary {
	attrs := uniqueAttributes(s)
	ocs := uniqueObjectClasses(s)
	return SchemaSummary{AttributeTypes: attrs, ObjectClasses: ocs, Warnings: s.Warnings}
}

func uniqueAttributes(s *schema.Schema) []schema.AttributeType {
	seen := map[string]bool{}
	var out []schema.AttributeType
	for _, attr := range s.AttributeTypes {
		key := schema.NormalizeName(attr.OID)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, attr)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].PrimaryName() < out[j].PrimaryName()
	})
	return out
}

func uniqueObjectClasses(s *schema.Schema) []ObjectClassSummary {
	seen := map[string]bool{}
	var out []ObjectClassSummary
	for _, oc := range s.ObjectClasses {
		key := schema.NormalizeName(oc.OID)
		if seen[key] {
			continue
		}
		seen[key] = true
		resolved, _ := s.ResolveObjectClasses([]string{oc.PrimaryName()})
		out = append(out, ObjectClassSummary{
			Name:     oc.PrimaryName(),
			OID:      oc.OID,
			Kind:     oc.Kind,
			SUP:      oc.SUP,
			Must:     resolved.Must,
			May:      resolved.May,
			Warnings: resolved.Warnings,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}
