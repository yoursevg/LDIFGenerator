package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yoursevg/LDIFGenerator/internal/ldapimport"
)

func main() {
	file := flag.String("file", "", "LDIF file to import")
	ldapaddPath := flag.String("ldapadd", "ldapadd", "ldapadd executable path")
	jobs := flag.Int("jobs", 4, "parallel ldapadd jobs for independent entry chunks")
	groupJobs := flag.Int("group-jobs", 1, "parallel ldapadd jobs for group chunks; keep 1 when nested groups or referential integrity are enabled")
	chunkRecords := flag.Int("chunk-records", 5000, "records per temporary LDIF chunk")
	workDir := flag.String("workdir", "", "directory for temporary LDIF chunks")
	keepChunks := flag.Bool("keep-chunks", false, "keep generated chunk files after import")
	splitOnly := flag.Bool("split-only", false, "only split LDIF into phased chunks, do not run ldapadd")
	flag.Parse()
	if *file == "" {
		flag.Usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *splitOnly {
		chunks, err := ldapimport.SplitFile(*file, ldapimport.SplitOptions{ChunkRecords: *chunkRecords, WorkDir: *workDir})
		if err != nil {
			log.Fatal(err)
		}
		records := 0
		for _, chunk := range chunks {
			records += chunk.Records
			fmt.Printf("%s\t%d records\t%s\n", ldapimport.PhaseName(chunk.Phase), chunk.Records, chunk.Path)
		}
		fmt.Printf("split %d records into %d chunks\n", records, len(chunks))
		return
	}

	report, err := ldapimport.Run(ctx, ldapimport.RunOptions{
		File:         *file,
		WorkDir:      *workDir,
		LDAPAddPath:  *ldapaddPath,
		LDAPAddArgs:  flag.Args(),
		Jobs:         *jobs,
		GroupJobs:    *groupJobs,
		ChunkRecords: *chunkRecords,
		KeepChunks:   *keepChunks,
		Progress: func(chunk ldapimport.Chunk, done, total int) {
			fmt.Printf("\r%d/%d chunks imported, last=%s records=%d", done, total, ldapimport.PhaseName(chunk.Phase), chunk.Records)
		},
	})
	fmt.Println()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("imported %d records from %d chunks in %s\n", report.Records, report.Chunks, report.Duration)
	if *keepChunks {
		fmt.Printf("chunks kept in %s\n", report.WorkDir)
	}
}
