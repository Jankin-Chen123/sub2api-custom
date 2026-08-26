package handler

import (
	"archive/zip"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"
)

func TestImportNotionArchiveConvertsDirectExport(t *testing.T) {
	markdown := `# 使用教程 39def5f577fd80d69d5bf000c023ef05

<aside>
💡

这是提示内容。
</aside>

- 第一章
    - 安装方法
        请按步骤安装。

        ![安装截图](image%201.png)

- 第二章
    这是第二章。
`
	archive := makeDocumentationZip(t, map[string][]byte{
		"使用教程 39def5f577fd80d69d5bf000c023ef05.md": []byte(markdown),
		"image 1.png": makeDocumentationPNG(t),
	})

	result, err := importNotionArchive(archive)
	if err != nil {
		t.Fatalf("import archive: %v", err)
	}
	content := string(result.Markdown)
	for _, expected := range []string{
		"# 使用教程",
		"> [!TIP]",
		"## 第一章",
		"### 安装方法",
		"![安装截图](assets/0001.png)",
		"## 第二章",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("converted Markdown missing %q:\n%s", expected, content)
		}
	}
	if strings.Contains(content, "<aside>") || strings.Contains(content, "39def5f577fd80d69d5bf000c023ef05") {
		t.Fatalf("Notion-only markup was not removed:\n%s", content)
	}
	if result.Title != "使用教程" {
		t.Fatalf("title = %q, want 使用教程", result.Title)
	}
	if len(result.Assets) != 1 || len(result.AssetMeta) != 1 {
		t.Fatalf("assets = %d/%d, want 1/1", len(result.Assets), len(result.AssetMeta))
	}
	if len(result.Outline) != 4 {
		t.Fatalf("outline length = %d, want 4", len(result.Outline))
	}
}

func TestImportNotionArchiveRejectsNestedZip(t *testing.T) {
	inner := makeDocumentationZip(t, map[string][]byte{"guide.md": []byte("# Guide\n")})
	outer := makeDocumentationZip(t, map[string][]byte{"Export-Part-1.zip": inner})

	_, err := importNotionArchive(outer)
	if !errors.Is(err, errDocumentationInvalidArchive) {
		t.Fatalf("error = %v, want invalid archive", err)
	}
	if !strings.Contains(err.Error(), "请先手动解压") {
		t.Fatalf("error should explain how to upload the inner ZIP: %v", err)
	}
}

func TestImportNotionArchiveRejectsTraversal(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	entry, err := writer.Create("../guide.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("# Guide\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = importNotionArchive(buffer.Bytes())
	if !errors.Is(err, errDocumentationInvalidArchive) {
		t.Fatalf("error = %v, want invalid archive", err)
	}
}

func TestDocumentationStorePublishAndRollback(t *testing.T) {
	store := NewDocumentationStore(t.TempDir())
	clock := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return clock }

	first, err := store.Import("first.zip", makeDocumentationZip(t, map[string][]byte{"guide.md": []byte("# First\n")}))
	if err != nil {
		t.Fatalf("import first: %v", err)
	}
	firstPublished, err := store.Publish(first.DraftID)
	if err != nil {
		t.Fatalf("publish first: %v", err)
	}

	clock = clock.Add(time.Minute)
	second, err := store.Import("second.zip", makeDocumentationZip(t, map[string][]byte{"guide.md": []byte("# Second\n")}))
	if err != nil {
		t.Fatalf("import second: %v", err)
	}
	if !second.Changes.HasActive || !second.Changes.ContentChanged {
		t.Fatalf("unexpected changes: %+v", second.Changes)
	}
	secondPublished, err := store.Publish(second.DraftID)
	if err != nil {
		t.Fatalf("publish second: %v", err)
	}
	active, err := store.ActiveManifest()
	if err != nil || active.ID != secondPublished.ID {
		t.Fatalf("active after second publish = %+v, %v", active, err)
	}

	clock = clock.Add(time.Minute)
	if _, err := store.Activate(firstPublished.ID); err != nil {
		t.Fatalf("activate first: %v", err)
	}
	active, err = store.ActiveManifest()
	if err != nil || active.ID != firstPublished.ID {
		t.Fatalf("active after rollback = %+v, %v", active, err)
	}
	content, err := store.VersionContent(firstPublished.ID)
	if err != nil || content != "# First\n" {
		t.Fatalf("first content = %q, %v", content, err)
	}
	state, err := store.State()
	if err != nil || len(state.Versions) != 2 || state.Active == nil || state.Active.ID != firstPublished.ID {
		t.Fatalf("unexpected state: %+v, %v", state, err)
	}
}

// Set NOTION_EXPORT_SAMPLE to exercise the importer against a real, innermost
// Notion Markdown export without checking a machine-specific sample into git.
func TestImportNotionArchiveRealSample(t *testing.T) {
	filename := os.Getenv("NOTION_EXPORT_SAMPLE")
	if filename == "" {
		t.Skip("NOTION_EXPORT_SAMPLE is not set")
	}
	archive, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	result, err := importNotionArchive(archive)
	if err != nil {
		t.Fatalf("import real sample: %v", err)
	}
	if result.Title == "" || len(result.Markdown) == 0 || len(result.Outline) == 0 {
		t.Fatalf("incomplete import result: title=%q markdown=%d outline=%d", result.Title, len(result.Markdown), len(result.Outline))
	}
	t.Logf("title=%q markdown=%d assets=%d outline=%d warnings=%d", result.Title, len(result.Markdown), len(result.Assets), len(result.Outline), len(result.Warnings))
}

func makeDocumentationZip(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create ZIP entry: %v", err)
		}
		if _, err := entry.Write(data); err != nil {
			t.Fatalf("write ZIP entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	return buffer.Bytes()
}

func makeDocumentationPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 80, G: 120, B: 240, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return buffer.Bytes()
}
