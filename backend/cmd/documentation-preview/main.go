// Command documentation-preview serves a real Notion export through the public
// documentation API without requiring PostgreSQL or Redis. It is intended for
// local visual QA together with the frontend Vite development server.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler"
)

type apiResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func main() {
	archivePath := flag.String("archive", "", "path to the innermost Notion export ZIP")
	listenAddress := flag.String("listen", "127.0.0.1:8080", "local listen address")
	flag.Parse()
	if strings.TrimSpace(*archivePath) == "" {
		log.Fatal("-archive is required")
	}

	archive, err := os.ReadFile(*archivePath)
	if err != nil {
		log.Fatalf("read archive: %v", err)
	}
	dataDir, err := os.MkdirTemp("", "sub2api-documentation-preview-")
	if err != nil {
		log.Fatalf("create preview directory: %v", err)
	}
	defer func() { _ = os.RemoveAll(dataDir) }()

	store := handler.NewDocumentationStore(dataDir)
	preview, err := store.Import(filepath.Base(*archivePath), archive)
	if err != nil {
		log.Fatalf("import archive: %v", err)
	}
	manifest, err := store.Publish(preview.DraftID)
	if err != nil {
		log.Fatalf("publish preview: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(writer http.ResponseWriter, _ *http.Request) {
		writeJSON(writer, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: map[string]string{"status": "ok"}})
	})
	docsHandler := func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/v1/docs" {
			writeJSON(writer, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: manifest})
			return
		}
		serveVersionRequest(writer, request, store, manifest.ID)
	}
	mux.HandleFunc("/api/v1/docs", docsHandler)
	mux.HandleFunc("/api/v1/docs/", docsHandler)

	server := &http.Server{Addr: *listenAddress, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("documentation preview ready: http://%s/docs (format=%s, images=%d, sections=%d)", *listenAddress, manifest.ContentFormat, len(manifest.Assets), len(manifest.Outline))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve preview: %v", err)
	}
}

func serveVersionRequest(writer http.ResponseWriter, request *http.Request, store *handler.DocumentationStore, activeID string) {
	prefix := "/api/v1/docs/versions/"
	if !strings.HasPrefix(request.URL.Path, prefix) {
		http.NotFound(writer, request)
		return
	}
	remainder := strings.TrimPrefix(request.URL.Path, prefix)
	parts := strings.SplitN(remainder, "/", 3)
	if len(parts) < 2 || parts[0] != activeID {
		http.NotFound(writer, request)
		return
	}
	switch parts[1] {
	case "content":
		content, _, err := store.VersionContent(activeID)
		if err != nil {
			http.Error(writer, "content unavailable", http.StatusNotFound)
			return
		}
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(content))
	case "assets":
		if len(parts) != 3 {
			http.NotFound(writer, request)
			return
		}
		assetPath, err := store.AssetPath("versions", activeID, parts[2])
		if err != nil {
			http.NotFound(writer, request)
			return
		}
		// AssetPath validates the request-derived filename, resolves symlinks, and
		// guarantees the result stays inside the version's assets directory.
		assetFile, err := os.Open(assetPath) // #nosec G703 -- validated by AssetPath
		if err != nil {
			http.NotFound(writer, request)
			return
		}
		defer func() { _ = assetFile.Close() }()
		assetInfo, err := assetFile.Stat()
		if err != nil {
			http.NotFound(writer, request)
			return
		}
		http.ServeContent(writer, request, filepath.Base(assetPath), assetInfo.ModTime(), assetFile)
	default:
		http.NotFound(writer, request)
	}
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		_, _ = fmt.Fprintln(writer, `{"code":500,"message":"encode response failed"}`)
	}
}
