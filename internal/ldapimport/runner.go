package ldapimport

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type RunOptions struct {
	File         string
	WorkDir      string
	LDAPAddPath  string
	LDAPAddArgs  []string
	Jobs         int
	GroupJobs    int
	ChunkRecords int
	KeepChunks   bool
	Progress     func(Chunk, int, int)
}

type Report struct {
	Chunks   int
	Records  int
	WorkDir  string
	Duration time.Duration
}

func Run(ctx context.Context, opts RunOptions) (Report, error) {
	if opts.LDAPAddPath == "" {
		opts.LDAPAddPath = "ldapadd"
	}
	if opts.Jobs <= 0 {
		opts.Jobs = 4
	}
	if opts.GroupJobs <= 0 {
		opts.GroupJobs = 1
	}
	start := time.Now()
	chunks, err := SplitFile(opts.File, SplitOptions{ChunkRecords: opts.ChunkRecords, WorkDir: opts.WorkDir})
	if err != nil {
		return Report{}, err
	}
	report := Report{Chunks: len(chunks), Duration: time.Since(start)}
	for _, chunk := range chunks {
		report.Records += chunk.Records
		report.WorkDir = dirOf(chunk.Path)
		break
	}
	if !opts.KeepChunks && report.WorkDir != "" {
		defer os.RemoveAll(report.WorkDir)
	}

	done := 0
	for _, phase := range []Phase{PhaseContainers, PhaseEntries, PhaseGroups} {
		phaseChunks := filterChunks(chunks, phase)
		if len(phaseChunks) == 0 {
			continue
		}
		jobs := opts.Jobs
		if phase == PhaseGroups {
			jobs = opts.GroupJobs
		}
		if err := runPhase(ctx, opts, phaseChunks, jobs, &done, len(chunks)); err != nil {
			return report, err
		}
	}
	report.Duration = time.Since(start)
	return report, nil
}

func runPhase(ctx context.Context, opts RunOptions, chunks []Chunk, jobs int, done *int, total int) error {
	if jobs <= 0 {
		jobs = 1
	}
	phaseCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	work := make(chan Chunk)
	errs := make(chan error, 1)
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for chunk := range work {
				if err := runLDAPAdd(phaseCtx, opts, chunk); err != nil {
					select {
					case errs <- err:
					default:
					}
					cancel()
					return
				}
				mu.Lock()
				*done = *done + 1
				current := *done
				mu.Unlock()
				if opts.Progress != nil {
					opts.Progress(chunk, current, total)
				}
			}
		}()
	}
	for _, chunk := range chunks {
		select {
		case err := <-errs:
			close(work)
			wg.Wait()
			return err
		case <-phaseCtx.Done():
			close(work)
			wg.Wait()
			select {
			case err := <-errs:
				return err
			default:
			}
			if err := phaseCtx.Err(); err != nil {
				return err
			}
			return ctx.Err()
		case work <- chunk:
		}
	}
	close(work)
	wg.Wait()
	select {
	case err := <-errs:
		return err
	default:
		return nil
	}
}

func runLDAPAdd(ctx context.Context, opts RunOptions, chunk Chunk) error {
	args := append([]string{}, opts.LDAPAddArgs...)
	args = append(args, "-f", chunk.Path)
	cmd := exec.CommandContext(ctx, opts.LDAPAddPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed for %s: %w\n%s", opts.LDAPAddPath, chunk.Path, err, string(output))
	}
	return nil
}

func filterChunks(chunks []Chunk, phase Phase) []Chunk {
	var out []Chunk
	for _, chunk := range chunks {
		if chunk.Phase == phase {
			out = append(out, chunk)
		}
	}
	return out
}

func dirOf(path string) string {
	return filepath.Dir(path)
}
