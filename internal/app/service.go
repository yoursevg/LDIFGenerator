package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	Written int `json:"written"`
	Total   int `json:"total"`
}

type Service struct {
	mu       sync.Mutex
	ctx      context.Context
	schema   *schema.Schema
	progress Progress
	cancel   context.CancelFunc
	report   generator.Report
	running  bool
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) SetContext(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx = ctx
}

func (s *Service) SelectSchemaFiles() ([]string, error) {
	s.mu.Lock()
	ctx := s.ctx
	s.mu.Unlock()
	if ctx == nil {
		return nil, fmt.Errorf("wails context is not ready")
	}
	return runtime.OpenMultipleFilesDialog(ctx, runtime.OpenDialogOptions{
		Title: "Select LDAP schema LDIF files",
		Filters: []runtime.FileFilter{
			{DisplayName: "LDIF and schema files", Pattern: "*.ldif;*.schema;*.conf"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
}

func (s *Service) SelectOutputPath() (string, error) {
	s.mu.Lock()
	ctx := s.ctx
	s.mu.Unlock()
	if ctx == nil {
		return "", fmt.Errorf("wails context is not ready")
	}
	return runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
		Title:           "Select output LDIF file",
		DefaultFilename: "generated.ldif",
		Filters: []runtime.FileFilter{
			{DisplayName: "LDIF files", Pattern: "*.ldif"},
			{DisplayName: "All files", Pattern: "*"},
		},
	})
}

func (s *Service) LoadSchema(paths []string) (SchemaSummary, error) {
	parsed, err := schema.ParseFiles(paths)
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
	return os.WriteFile(path, data, 0o644)
}

func (s *Service) Generate(cfg generator.GeneratorConfig) (generator.Report, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return generator.Report{}, fmt.Errorf("generation is already running")
	}
	if s.schema == nil {
		s.mu.Unlock()
		return generator.Report{}, fmt.Errorf("schema is not loaded")
	}
	if cfg.OutputPath == "" {
		s.mu.Unlock()
		return generator.Report{}, fmt.Errorf("output LDIF path is required")
	}
	genSchema := s.schema
	baseCtx := s.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(baseCtx)
	s.cancel = cancel
	s.running = true
	s.progress = Progress{Total: cfg.Count}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.cancel = nil
		s.mu.Unlock()
	}()
	report, err := generator.New(genSchema).Generate(ctx, cfg, func(written, total int) {
		s.mu.Lock()
		s.progress = Progress{Written: written, Total: total}
		s.mu.Unlock()
	})
	s.mu.Lock()
	s.report = report
	s.mu.Unlock()
	return report, err
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
	return s.progress
}

func (s *Service) LastReport() generator.Report {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.report
}

func (s *Service) DefaultConfig() generator.GeneratorConfig {
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
	return out
}
