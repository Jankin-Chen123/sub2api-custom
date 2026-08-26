package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var documentationIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var errDocumentationNotFound = errors.New("documentation not found")

type DocumentationManifest struct {
	ID            string                 `json:"id"`
	Title         string                 `json:"title"`
	SourceFile    string                 `json:"source_file"`
	ContentFormat string                 `json:"content_format"`
	CreatedAt     time.Time              `json:"created_at"`
	PublishedAt   *time.Time             `json:"published_at,omitempty"`
	ContentSHA256 string                 `json:"content_sha256"`
	ContentBytes  int64                  `json:"content_bytes"`
	Assets        []DocumentationAsset   `json:"assets"`
	Outline       []DocumentationHeading `json:"outline"`
	Warnings      []string               `json:"warnings"`
}

type DocumentationChanges struct {
	HasActive      bool `json:"has_active"`
	ContentChanged bool `json:"content_changed"`
	AssetsAdded    int  `json:"assets_added"`
	AssetsRemoved  int  `json:"assets_removed"`
	AssetsChanged  int  `json:"assets_changed"`
}

type DocumentationPreview struct {
	DraftID  string                `json:"draft_id"`
	Manifest DocumentationManifest `json:"manifest"`
	Content  string                `json:"content"`
	Markdown string                `json:"markdown,omitempty"`
	Changes  DocumentationChanges  `json:"changes"`
}

type DocumentationVersion struct {
	Manifest DocumentationManifest `json:"manifest"`
	Active   bool                  `json:"active"`
}

type DocumentationState struct {
	Active   *DocumentationManifest `json:"active,omitempty"`
	Versions []DocumentationVersion `json:"versions"`
}

type documentationActivation struct {
	VersionID   string    `json:"version_id"`
	ActivatedAt time.Time `json:"activated_at"`
}

type DocumentationStore struct {
	root string
	now  func() time.Time
	mu   sync.RWMutex
}

func NewDocumentationStore(dataDir string) *DocumentationStore {
	return &DocumentationStore{root: filepath.Join(dataDir, "documentation"), now: time.Now}
}

func (s *DocumentationStore) ensureDirectories() error {
	for _, name := range []string{"drafts", "versions", "activations"} {
		if err := os.MkdirAll(filepath.Join(s.root, name), 0755); err != nil {
			return err
		}
	}
	return nil
}

func (s *DocumentationStore) Import(sourceFile string, archive []byte) (*DocumentationPreview, error) {
	result, err := importNotionArchive(archive)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDirectories(); err != nil {
		return nil, err
	}
	_ = s.cleanupExpiredDraftsLocked(24 * time.Hour)

	id := uuid.NewString()
	createdAt := s.now().UTC()
	digest := sha256.Sum256(result.Content)
	manifest := DocumentationManifest{
		ID: id, Title: result.Title, SourceFile: filepath.Base(sourceFile), CreatedAt: createdAt,
		ContentFormat: result.Format, ContentSHA256: hex.EncodeToString(digest[:]), ContentBytes: int64(len(result.Content)),
		Assets: result.AssetMeta, Outline: result.Outline, Warnings: result.Warnings,
	}

	tempDir := filepath.Join(s.root, "drafts", ".tmp-"+id)
	draftDir := filepath.Join(s.root, "drafts", id)
	if err := os.MkdirAll(filepath.Join(tempDir, "assets"), 0755); err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tempDir) }()
	if err := os.WriteFile(filepath.Join(tempDir, documentationContentFilename(manifest)), result.Content, 0644); err != nil {
		return nil, err
	}
	for assetPath, data := range result.Assets {
		name := filepath.Base(filepath.FromSlash(assetPath))
		if err := os.WriteFile(filepath.Join(tempDir, "assets", name), data, 0644); err != nil {
			return nil, err
		}
	}
	if err := writeDocumentationJSON(filepath.Join(tempDir, "manifest.json"), manifest); err != nil {
		return nil, err
	}
	if err := os.Rename(tempDir, draftDir); err != nil {
		return nil, err
	}

	changes := DocumentationChanges{ContentChanged: true, AssetsAdded: len(manifest.Assets)}
	if active, err := s.activeManifestLocked(); err == nil {
		changes = compareDocumentationManifests(active, &manifest)
	} else if !errors.Is(err, errDocumentationNotFound) {
		return nil, err
	}

	preview := &DocumentationPreview{DraftID: id, Manifest: manifest, Content: string(result.Content), Changes: changes}
	if result.Format == documentationFormatMarkdown {
		preview.Markdown = preview.Content
	}
	return preview, nil
}

func (s *DocumentationStore) Publish(draftID string) (*DocumentationManifest, error) {
	if !documentationIDPattern.MatchString(draftID) {
		return nil, errDocumentationNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureDirectories(); err != nil {
		return nil, err
	}

	draftDir := filepath.Join(s.root, "drafts", draftID)
	manifest, err := readDocumentationManifest(filepath.Join(draftDir, "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errDocumentationNotFound
		}
		return nil, err
	}
	publishedAt := s.now().UTC()
	manifest.PublishedAt = &publishedAt
	if err := writeDocumentationJSON(filepath.Join(draftDir, "manifest.json"), manifest); err != nil {
		return nil, err
	}
	versionDir := filepath.Join(s.root, "versions", draftID)
	if _, err := os.Stat(versionDir); err == nil {
		return nil, fmt.Errorf("documentation version already exists")
	}
	if err := os.Rename(draftDir, versionDir); err != nil {
		return nil, err
	}
	if err := s.activateLocked(draftID, publishedAt); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (s *DocumentationStore) Activate(versionID string) (*DocumentationManifest, error) {
	if !documentationIDPattern.MatchString(versionID) {
		return nil, errDocumentationNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	manifest, err := readDocumentationManifest(filepath.Join(s.root, "versions", versionID, "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errDocumentationNotFound
		}
		return nil, err
	}
	if err := s.activateLocked(versionID, s.now().UTC()); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (s *DocumentationStore) activateLocked(versionID string, activatedAt time.Time) error {
	activation := documentationActivation{VersionID: versionID, ActivatedAt: activatedAt}
	name := activatedAt.UTC().Format("20060102T150405.000000000Z") + "-" + uuid.NewString() + ".json"
	tempPath := filepath.Join(s.root, "activations", ".tmp-"+name)
	finalPath := filepath.Join(s.root, "activations", name)
	if err := writeDocumentationJSON(tempPath, activation); err != nil {
		return err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		_ = os.Remove(tempPath)
		return err
	}
	return nil
}

func (s *DocumentationStore) State() (*DocumentationState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.ensureDirectories(); err != nil {
		return nil, err
	}
	activeID, _ := s.activeVersionIDLocked()
	entries, err := os.ReadDir(filepath.Join(s.root, "versions"))
	if err != nil {
		return nil, err
	}
	state := &DocumentationState{Versions: make([]DocumentationVersion, 0)}
	for _, entry := range entries {
		if !entry.IsDir() || !documentationIDPattern.MatchString(entry.Name()) {
			continue
		}
		manifest, err := readDocumentationManifest(filepath.Join(s.root, "versions", entry.Name(), "manifest.json"))
		if err != nil {
			continue
		}
		active := manifest.ID == activeID
		state.Versions = append(state.Versions, DocumentationVersion{Manifest: manifest, Active: active})
		if active {
			copy := manifest
			state.Active = &copy
		}
	}
	sort.Slice(state.Versions, func(i, j int) bool {
		return state.Versions[i].Manifest.CreatedAt.After(state.Versions[j].Manifest.CreatedAt)
	})
	return state, nil
}

func (s *DocumentationStore) ActiveManifest() (*DocumentationManifest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeManifestLocked()
}

func (s *DocumentationStore) activeManifestLocked() (*DocumentationManifest, error) {
	id, err := s.activeVersionIDLocked()
	if err != nil {
		return nil, err
	}
	manifest, err := readDocumentationManifest(filepath.Join(s.root, "versions", id, "manifest.json"))
	if os.IsNotExist(err) {
		return nil, errDocumentationNotFound
	}
	return &manifest, err
}

func (s *DocumentationStore) activeVersionIDLocked() (string, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "activations"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", errDocumentationNotFound
		}
		return "", err
	}
	var latest documentationActivation
	found := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), ".tmp-") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.root, "activations", entry.Name()))
		if err != nil {
			continue
		}
		var activation documentationActivation
		if json.Unmarshal(data, &activation) != nil || !documentationIDPattern.MatchString(activation.VersionID) {
			continue
		}
		if !found || activation.ActivatedAt.After(latest.ActivatedAt) {
			latest = activation
			found = true
		}
	}
	if !found {
		return "", errDocumentationNotFound
	}
	return latest.VersionID, nil
}

func (s *DocumentationStore) VersionContent(versionID string) (string, string, error) {
	if !documentationIDPattern.MatchString(versionID) {
		return "", "", errDocumentationNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	manifest, err := readDocumentationManifest(filepath.Join(s.root, "versions", versionID, "manifest.json"))
	if os.IsNotExist(err) {
		return "", "", errDocumentationNotFound
	}
	if err != nil {
		return "", "", err
	}
	data, err := os.ReadFile(filepath.Join(s.root, "versions", versionID, documentationContentFilename(manifest)))
	if os.IsNotExist(err) {
		return "", "", errDocumentationNotFound
	}
	if err != nil {
		return "", "", err
	}
	if len(data) > maxDocumentationContentBytes {
		return "", "", errDocumentationArchiveTooLarge
	}
	return string(data), manifest.ContentFormat, nil
}

func (s *DocumentationStore) AssetPath(kind, id, filename string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !documentationIDPattern.MatchString(id) {
		return "", errDocumentationNotFound
	}
	rel, ok := cleanPageImageRelativePath(filename)
	if !ok || filepath.Dir(rel) != "." {
		return "", errDocumentationNotFound
	}
	var base string
	switch kind {
	case "versions":
		base = filepath.Join(s.root, "versions", id, "assets")
	case "drafts":
		base = filepath.Join(s.root, "drafts", id, "assets")
		manifest, err := readDocumentationManifest(filepath.Join(s.root, "drafts", id, "manifest.json"))
		if err != nil || s.now().After(manifest.CreatedAt.Add(24*time.Hour)) {
			return "", errDocumentationNotFound
		}
	default:
		return "", errDocumentationNotFound
	}
	resolved, ok := resolvePageImagePath(s.root, base, rel)
	if !ok {
		return "", errDocumentationNotFound
	}
	return resolved, nil
}

func (s *DocumentationStore) cleanupExpiredDraftsLocked(maxAge time.Duration) error {
	entries, err := os.ReadDir(filepath.Join(s.root, "drafts"))
	if err != nil {
		return err
	}
	cutoff := s.now().Add(-maxAge)
	for _, entry := range entries {
		if !entry.IsDir() || !documentationIDPattern.MatchString(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(s.root, "drafts", entry.Name()))
		}
	}
	return nil
}

func compareDocumentationManifests(active, next *DocumentationManifest) DocumentationChanges {
	changes := DocumentationChanges{HasActive: active != nil, ContentChanged: active == nil || active.ContentSHA256 != next.ContentSHA256}
	oldAssets := make(map[string]string, len(active.Assets))
	for _, asset := range active.Assets {
		oldAssets[asset.Path] = asset.SHA256
	}
	newAssets := make(map[string]string, len(next.Assets))
	for _, asset := range next.Assets {
		newAssets[asset.Path] = asset.SHA256
		oldDigest, exists := oldAssets[asset.Path]
		if !exists {
			changes.AssetsAdded++
		} else if oldDigest != asset.SHA256 {
			changes.AssetsChanged++
		}
	}
	for name := range oldAssets {
		if _, exists := newAssets[name]; !exists {
			changes.AssetsRemoved++
		}
	}
	return changes
}

func writeDocumentationJSON(filename string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, append(data, '\n'), 0644)
}

func readDocumentationManifest(filename string) (DocumentationManifest, error) {
	var manifest DocumentationManifest
	data, err := os.ReadFile(filename)
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	if !documentationIDPattern.MatchString(manifest.ID) {
		return manifest, fmt.Errorf("invalid documentation manifest")
	}
	if manifest.ContentFormat == "" {
		manifest.ContentFormat = documentationFormatMarkdown
	}
	if manifest.ContentFormat != documentationFormatMarkdown && manifest.ContentFormat != documentationFormatHTML {
		return manifest, fmt.Errorf("invalid documentation content format")
	}
	return manifest, nil
}

func documentationContentFilename(manifest DocumentationManifest) string {
	if manifest.ContentFormat == documentationFormatHTML {
		return "content.html"
	}
	return "content.md"
}

type DocumentationHandler struct {
	store *DocumentationStore
}

func NewDocumentationHandler(dataDir string) *DocumentationHandler {
	return &DocumentationHandler{store: NewDocumentationStore(dataDir)}
}

func (h *DocumentationHandler) Import(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxDocumentationArchiveBytes+(1<<20))
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "请选择 Notion 导出的 ZIP 文件")
		return
	}
	defer func() { _ = file.Close() }()
	if c.Request.MultipartForm != nil {
		defer func() { _ = c.Request.MultipartForm.RemoveAll() }()
	}
	if !strings.EqualFold(filepath.Ext(header.Filename), ".zip") {
		response.BadRequest(c, "只支持 ZIP 文件")
		return
	}
	archive, err := io.ReadAll(io.LimitReader(file, maxDocumentationArchiveBytes+1))
	if err != nil || len(archive) > maxDocumentationArchiveBytes {
		response.Error(c, http.StatusRequestEntityTooLarge, "ZIP 文件不能超过 64 MB")
		return
	}
	preview, err := h.store.Import(header.Filename, archive)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Created(c, preview)
}

func (h *DocumentationHandler) Publish(c *gin.Context) {
	manifest, err := h.store.Publish(c.Param("draftID"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, manifest)
}

func (h *DocumentationHandler) Activate(c *gin.Context) {
	manifest, err := h.store.Activate(c.Param("versionID"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, manifest)
}

func (h *DocumentationHandler) State(c *gin.Context) {
	state, err := h.store.State()
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, state)
}

func (h *DocumentationHandler) Active(c *gin.Context) {
	manifest, err := h.store.ActiveManifest()
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Success(c, manifest)
}

func (h *DocumentationHandler) VersionContent(c *gin.Context) {
	content, format, err := h.store.VersionContent(c.Param("versionID"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	contentType := "text/markdown; charset=utf-8"
	if format == documentationFormatHTML {
		contentType = "text/plain; charset=utf-8"
	}
	c.Data(http.StatusOK, contentType, []byte(content))
}

func (h *DocumentationHandler) VersionAsset(c *gin.Context) {
	h.serveAsset(c, "versions", c.Param("versionID"))
}

func (h *DocumentationHandler) DraftAsset(c *gin.Context) {
	h.serveAsset(c, "drafts", c.Param("draftID"))
}

func (h *DocumentationHandler) serveAsset(c *gin.Context, kind, id string) {
	filename := strings.TrimPrefix(c.Param("filename"), "/")
	resolved, err := h.store.AssetPath(kind, id, filename)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if kind == "versions" {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "no-store")
	}
	c.File(resolved)
}

func (h *DocumentationHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errDocumentationNotFound):
		response.NotFound(c, "文档或版本不存在")
	case errors.Is(err, errDocumentationArchiveTooLarge):
		response.Error(c, http.StatusRequestEntityTooLarge, err.Error())
	case errors.Is(err, errDocumentationInvalidArchive), errors.Is(err, errDocumentationNoContent):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, "文档处理失败")
	}
}
