package ldapimport

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Phase int

const (
	PhaseContainers Phase = iota
	PhaseEntries
	PhaseGroups
)

type Chunk struct {
	Path    string
	Phase   Phase
	Index   int
	Records int
}

type SplitOptions struct {
	ChunkRecords int
	WorkDir      string
}

func SplitFile(path string, opts SplitOptions) ([]Chunk, error) {
	if opts.ChunkRecords <= 0 {
		opts.ChunkRecords = 5000
	}
	if opts.WorkDir == "" {
		dir, err := os.MkdirTemp("", "ldifgenerator-ldapadd-*")
		if err != nil {
			return nil, err
		}
		opts.WorkDir = dir
	} else if err := os.MkdirAll(opts.WorkDir, 0o755); err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	writers := map[Phase]*chunkWriter{}
	var chunks []Chunk
	err = scanRawRecords(f, func(raw string) error {
		phase := classifyRecord(raw)
		cw := writers[phase]
		if cw == nil {
			cw = &chunkWriter{phase: phase, dir: opts.WorkDir, chunkRecords: opts.ChunkRecords}
			writers[phase] = cw
		}
		chunk, err := cw.write(raw)
		if err != nil {
			return err
		}
		if chunk != nil {
			chunks = append(chunks, *chunk)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, phase := range []Phase{PhaseContainers, PhaseEntries, PhaseGroups} {
		if writers[phase] == nil {
			continue
		}
		chunk, err := writers[phase].close()
		if err != nil {
			return nil, err
		}
		if chunk != nil {
			chunks = append(chunks, *chunk)
		}
	}
	sort.Slice(chunks, func(i, j int) bool {
		if chunks[i].Phase != chunks[j].Phase {
			return chunks[i].Phase < chunks[j].Phase
		}
		return chunks[i].Index < chunks[j].Index
	})
	return chunks, nil
}

func PhaseName(phase Phase) string {
	switch phase {
	case PhaseContainers:
		return "containers"
	case PhaseEntries:
		return "entries"
	case PhaseGroups:
		return "groups"
	default:
		return "unknown"
	}
}

type chunkWriter struct {
	phase        Phase
	dir          string
	chunkRecords int
	index        int
	records      int
	file         *os.File
	writer       *bufio.Writer
}

func (w *chunkWriter) write(raw string) (*Chunk, error) {
	if w.file == nil {
		if err := w.openNext(); err != nil {
			return nil, err
		}
	}
	if _, err := w.writer.WriteString(raw); err != nil {
		return nil, err
	}
	if !strings.HasSuffix(raw, "\n\n") {
		if !strings.HasSuffix(raw, "\n") {
			if err := w.writer.WriteByte('\n'); err != nil {
				return nil, err
			}
		}
		if err := w.writer.WriteByte('\n'); err != nil {
			return nil, err
		}
	}
	w.records++
	if w.records < w.chunkRecords {
		return nil, nil
	}
	return w.close()
}

func (w *chunkWriter) openNext() error {
	name := fmt.Sprintf("%02d-%s-%06d.ldif", w.phase, PhaseName(w.phase), w.index)
	file, err := os.Create(filepath.Join(w.dir, name))
	if err != nil {
		return err
	}
	w.file = file
	w.writer = bufio.NewWriterSize(file, 1024*1024)
	w.records = 0
	return nil
}

func (w *chunkWriter) close() (*Chunk, error) {
	if w.file == nil {
		return nil, nil
	}
	if err := w.writer.Flush(); err != nil {
		_ = w.file.Close()
		return nil, err
	}
	path := w.file.Name()
	if err := w.file.Close(); err != nil {
		return nil, err
	}
	chunk := &Chunk{Path: path, Phase: w.phase, Index: w.index, Records: w.records}
	w.file = nil
	w.writer = nil
	w.records = 0
	w.index++
	return chunk, nil
}

func scanRawRecords(r io.Reader, fn func(string) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)
	var b strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			if b.Len() == 0 {
				continue
			}
			b.WriteString("\n\n")
			if err := fn(b.String()); err != nil {
				return err
			}
			b.Reset()
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if b.Len() > 0 {
		return fn(b.String())
	}
	return nil
}

func classifyRecord(raw string) Phase {
	attrs := parseAttrs(raw)
	classes := attrs["objectclass"]
	if hasAnyAttr(attrs, "member", "uniquemember") || containsClass(classes, "group") {
		return PhaseGroups
	}
	if containsAnyClass(classes, "organizationalunit", "organization", "dcobject", "domain", "container") {
		return PhaseContainers
	}
	return PhaseEntries
}

func parseAttrs(raw string) map[string][]string {
	attrs := map[string][]string{}
	var current string
	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, " ") {
			if current != "" {
				last := len(attrs[current]) - 1
				attrs[current][last] += strings.TrimPrefix(line, " ")
			}
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		current = strings.ToLower(strings.TrimSpace(name))
		value = strings.TrimSpace(strings.TrimPrefix(value, ":"))
		attrs[current] = append(attrs[current], value)
	}
	return attrs
}

func hasAnyAttr(attrs map[string][]string, names ...string) bool {
	for _, name := range names {
		if len(attrs[name]) > 0 {
			return true
		}
	}
	return false
}

func containsAnyClass(classes []string, values ...string) bool {
	for _, value := range values {
		if containsClass(classes, value) {
			return true
		}
	}
	return false
}

func containsClass(classes []string, value string) bool {
	value = strings.ToLower(value)
	for _, class := range classes {
		if strings.Contains(strings.ToLower(class), value) {
			return true
		}
	}
	return false
}
