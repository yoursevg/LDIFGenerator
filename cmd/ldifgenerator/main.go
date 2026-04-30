package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/yoursevg/LDIFGenerator/internal/generator"
	"github.com/yoursevg/LDIFGenerator/internal/schema"
)

func main() {
	schemaPath := flag.String("schema", "", "comma-separated schema LDIF files or directories")
	configPath := flag.String("config", "", "generation config JSON")
	flag.Parse()
	if *schemaPath == "" || *configPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	paths, err := schema.ResolveInputPaths(splitCSV(*schemaPath))
	if err != nil {
		log.Fatal(err)
	}
	s, err := schema.ParseFiles(paths)
	if err != nil {
		log.Fatal(err)
	}
	var cfg generator.GeneratorConfig
	data, err := os.ReadFile(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Fatal(err)
	}
	report, err := generator.New(s).Generate(context.Background(), cfg, func(done, total int) {
		fmt.Printf("\r%d/%d", done, total)
	})
	fmt.Println()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("generated %d records in %s (%.0f records/sec), %d bytes\n", report.Records, report.Duration, report.RecordsPerSec, report.FileBytes)
}

func splitCSV(s string) []string {
	var out []string
	start := 0
	for i, r := range s {
		if r == ',' {
			if start < i {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
