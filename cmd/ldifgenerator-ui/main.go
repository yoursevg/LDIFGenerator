package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yoursevg/LDIFGenerator/internal/app"
	"github.com/yoursevg/LDIFGenerator/internal/generator"
)

type schemaRequest struct {
	Paths []string `json:"paths"`
}

type configPathRequest struct {
	Path   string                    `json:"path"`
	Config generator.GeneratorConfig `json:"config"`
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	staticDir := flag.String("static", "frontend/dist", "frontend dist directory")
	flag.Parse()

	service := app.NewService()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/default-config", handleJSON(func(_ *http.Request) (generator.GeneratorConfig, error) {
		return app.DefaultConfig(), nil
	}))
	mux.HandleFunc("/api/schema/load", handleJSON(func(r *http.Request) (app.SchemaSummary, error) {
		var req schemaRequest
		if err := decodeJSON(r, &req); err != nil {
			return app.SchemaSummary{}, err
		}
		return service.LoadSchema(req.Paths)
	}))
	mux.HandleFunc("/api/config/load", handleJSON(func(r *http.Request) (generator.GeneratorConfig, error) {
		var req configPathRequest
		if err := decodeJSON(r, &req); err != nil {
			return generator.GeneratorConfig{}, err
		}
		return service.LoadConfig(req.Path)
	}))
	mux.HandleFunc("/api/config/save", handleJSON(func(r *http.Request) (map[string]bool, error) {
		var req configPathRequest
		if err := decodeJSON(r, &req); err != nil {
			return nil, err
		}
		if err := service.SaveConfig(req.Path, req.Config); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	}))
	mux.HandleFunc("/api/generate", handleJSON(func(r *http.Request) (app.Progress, error) {
		var cfg generator.GeneratorConfig
		if err := decodeJSON(r, &cfg); err != nil {
			return app.Progress{}, err
		}
		return service.StartGenerate(cfg)
	}))
	mux.HandleFunc("/api/cancel", handleJSON(func(_ *http.Request) (map[string]bool, error) {
		service.CancelGeneration()
		return map[string]bool{"ok": true}, nil
	}))
	mux.HandleFunc("/api/progress", handleJSON(func(_ *http.Request) (app.Progress, error) {
		return service.Progress(), nil
	}))
	mux.Handle("/", staticHandler(*staticDir))

	server := &http.Server{
		Addr:              *addr,
		Handler:           withCORS(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("LDIFGenerator UI backend listening on http://%s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func handleJSON[T any](fn func(*http.Request) (T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		value, err := fn(r)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(value); err != nil {
			log.Printf("write response: %v", err)
		}
	}
}

func decodeJSON(r *http.Request, out any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is required")
	}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		return err
	}
	return nil
}

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		next.ServeHTTP(w, r)
	})
}

func staticHandler(staticDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if _, err := os.Stat(staticDir); err != nil {
			http.Error(w, "frontend dist not found; run npm run build in frontend or use npm run dev", http.StatusNotFound)
			return
		}
		cleanPath := strings.TrimPrefix(filepath.Clean(r.URL.Path), string(filepath.Separator))
		path := filepath.Join(staticDir, cleanPath)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			http.ServeFile(w, r, path)
			return
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
}
