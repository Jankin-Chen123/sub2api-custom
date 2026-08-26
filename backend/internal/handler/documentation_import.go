package handler

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	_ "golang.org/x/image/webp"
)

const (
	maxDocumentationArchiveBytes        = 64 << 20
	maxDocumentationExpandedBytes       = 256 << 20
	maxDocumentationEntryBytes          = 32 << 20
	maxDocumentationEntries             = 5000
	maxDocumentationMarkdownBytes       = 4 << 20
	maxDocumentationImagePixels   int64 = 60_000_000
)

var (
	markdownImagePattern = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\r\n]+)\)`)
	sectionBulletPattern = regexp.MustCompile(`^(\s*)-\s+(.+?)\s*$`)
	headingPattern       = regexp.MustCompile(`^(#{1,4})\s+(.+?)\s*$`)
	notionIDPattern      = regexp.MustCompile(`(?i)\s+[0-9a-f]{32}$`)
	notionLinkPattern    = regexp.MustCompile(`(?i)https?://(?:www\.)?(?:app\.)?notion\.(?:so|com)/`)
)

var (
	errDocumentationInvalidArchive  = errors.New("invalid Notion export archive")
	errDocumentationArchiveTooLarge = errors.New("notion export archive is too large")
	errDocumentationNoMarkdown      = errors.New("notion export does not contain a Markdown file")
)

type documentationArchiveFile struct {
	Path string
	Data []byte
}

type documentationArchiveStats struct {
	entries       int
	expandedBytes int64
	warnings      []string
}

type DocumentationHeading struct {
	Level int    `json:"level"`
	Title string `json:"title"`
	ID    string `json:"id"`
}

type DocumentationAsset struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type documentationImportResult struct {
	Title     string
	Markdown  []byte
	Assets    map[string][]byte
	AssetMeta []DocumentationAsset
	Outline   []DocumentationHeading
	Warnings  []string
}

func importNotionArchive(data []byte) (*documentationImportResult, error) {
	if len(data) == 0 || len(data) > maxDocumentationArchiveBytes {
		return nil, errDocumentationArchiveTooLarge
	}

	files := make(map[string]documentationArchiveFile)
	stats := &documentationArchiveStats{}
	if err := collectDocumentationZip(data, stats, files); err != nil {
		return nil, err
	}

	markdownPaths := make([]string, 0)
	for name := range files {
		if strings.EqualFold(path.Ext(name), ".md") {
			markdownPaths = append(markdownPaths, name)
		}
	}
	if len(markdownPaths) == 0 {
		return nil, errDocumentationNoMarkdown
	}
	sort.Strings(markdownPaths)

	assetBySource := make(map[string]string)
	assets := make(map[string][]byte)
	assetMeta := make([]DocumentationAsset, 0)
	warnings := append([]string(nil), stats.warnings...)
	documents := make([]string, 0, len(markdownPaths))
	title := "Documentation"

	for index, markdownPath := range markdownPaths {
		file := files[markdownPath]
		if len(file.Data) > maxDocumentationMarkdownBytes {
			return nil, fmt.Errorf("%w: Markdown file %q exceeds %d bytes", errDocumentationArchiveTooLarge, markdownPath, maxDocumentationMarkdownBytes)
		}
		if !utf8.Valid(file.Data) {
			return nil, fmt.Errorf("%w: Markdown file %q is not UTF-8", errDocumentationInvalidArchive, markdownPath)
		}

		converted, docTitle, docWarnings, err := convertNotionMarkdown(
			string(file.Data),
			markdownPath,
			files,
			assetBySource,
			assets,
			&assetMeta,
		)
		if err != nil {
			return nil, err
		}
		warnings = append(warnings, docWarnings...)
		if index == 0 && docTitle != "" {
			title = docTitle
		}
		if len(markdownPaths) > 1 && index > 0 {
			converted = demoteDocumentTitle(converted)
		}
		documents = append(documents, strings.TrimSpace(converted))
	}

	markdown := strings.Join(documents, "\n\n---\n\n") + "\n"
	outline := extractDocumentationOutline(markdown)
	warnings = uniqueStrings(warnings)
	sort.Strings(warnings)

	return &documentationImportResult{
		Title:     title,
		Markdown:  []byte(markdown),
		Assets:    assets,
		AssetMeta: assetMeta,
		Outline:   outline,
		Warnings:  warnings,
	}, nil
}

func collectDocumentationZip(data []byte, stats *documentationArchiveStats, files map[string]documentationArchiveFile) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("%w: %v", errDocumentationInvalidArchive, err)
	}

	for _, entry := range reader.File {
		stats.entries++
		if stats.entries > maxDocumentationEntries {
			return fmt.Errorf("%w: archive contains more than %d entries", errDocumentationArchiveTooLarge, maxDocumentationEntries)
		}
		if entry.FileInfo().Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: symbolic links are not allowed", errDocumentationInvalidArchive)
		}
		if entry.FileInfo().IsDir() {
			continue
		}

		cleanName, ok := cleanDocumentationArchivePath(entry.Name)
		if !ok {
			return fmt.Errorf("%w: unsafe archive path %q", errDocumentationInvalidArchive, entry.Name)
		}
		if entry.UncompressedSize64 > maxDocumentationEntryBytes {
			return fmt.Errorf("%w: entry %q exceeds %d bytes", errDocumentationArchiveTooLarge, entry.Name, maxDocumentationEntryBytes)
		}
		if strings.EqualFold(path.Ext(cleanName), ".zip") {
			return fmt.Errorf("%w: ZIP 中仍包含 ZIP 文件 %q，请先手动解压并上传最内层的 Notion 导出包", errDocumentationInvalidArchive, cleanName)
		}

		content, err := readDocumentationZipEntry(entry)
		if err != nil {
			return err
		}
		stats.expandedBytes += int64(len(content))
		if stats.expandedBytes > maxDocumentationExpandedBytes {
			return fmt.Errorf("%w: expanded content exceeds %d bytes", errDocumentationArchiveTooLarge, maxDocumentationExpandedBytes)
		}

		virtualPath := cleanName
		ext := strings.ToLower(path.Ext(cleanName))
		if ext != ".md" && !isDocumentationImageExtension(ext) {
			stats.warnings = append(stats.warnings, fmt.Sprintf("已忽略不支持的文件：%s", virtualPath))
			continue
		}
		key := canonicalDocumentationPath(virtualPath)
		if _, exists := files[key]; exists {
			return fmt.Errorf("%w: duplicate archive path %q", errDocumentationInvalidArchive, virtualPath)
		}
		files[key] = documentationArchiveFile{Path: key, Data: content}
	}
	return nil
}

func readDocumentationZipEntry(entry *zip.File) ([]byte, error) {
	r, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: cannot open %q: %v", errDocumentationInvalidArchive, entry.Name, err)
	}
	defer func() { _ = r.Close() }()

	limited := io.LimitReader(r, maxDocumentationEntryBytes+1)
	content, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot read %q: %v", errDocumentationInvalidArchive, entry.Name, err)
	}
	if len(content) > maxDocumentationEntryBytes {
		return nil, fmt.Errorf("%w: entry %q exceeds %d bytes", errDocumentationArchiveTooLarge, entry.Name, maxDocumentationEntryBytes)
	}
	return content, nil
}

func cleanDocumentationArchivePath(name string) (string, bool) {
	if name == "" || strings.Contains(name, "\\") || strings.ContainsRune(name, 0) || strings.HasPrefix(name, "/") {
		return "", false
	}
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, ":") {
		return "", false
	}
	return cleaned, true
}

func canonicalDocumentationPath(name string) string {
	return strings.TrimPrefix(path.Clean(strings.ReplaceAll(name, "\\", "/")), "./")
}

func isDocumentationImageExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return true
	default:
		return false
	}
}

func convertNotionMarkdown(raw, markdownPath string, files map[string]documentationArchiveFile, assetBySource map[string]string, assets map[string][]byte, assetMeta *[]DocumentationAsset) (string, string, []string, error) {
	markdown := strings.ReplaceAll(raw, "\r\n", "\n")
	markdown = convertNotionSectionLists(markdown)
	markdown = convertNotionAsides(markdown)
	markdown = stripNotionIDsFromHeadings(markdown)
	title := firstDocumentationTitle(markdown)
	if title == "" {
		title = strings.TrimSuffix(path.Base(markdownPath), path.Ext(markdownPath))
		title = notionIDPattern.ReplaceAllString(title, "")
	}

	warnings := make([]string, 0)
	markdown = markdownImagePattern.ReplaceAllStringFunc(markdown, func(match string) string {
		parts := markdownImagePattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		rawTarget := strings.TrimSpace(parts[2])
		if strings.HasPrefix(rawTarget, "<") && strings.HasSuffix(rawTarget, ">") {
			rawTarget = strings.TrimSuffix(strings.TrimPrefix(rawTarget, "<"), ">")
		}
		if parsed, err := url.PathUnescape(rawTarget); err == nil {
			rawTarget = parsed
		}
		if strings.Contains(rawTarget, "://") || strings.HasPrefix(rawTarget, "data:") || strings.HasPrefix(rawTarget, "/") {
			return match
		}
		if cut := strings.IndexAny(rawTarget, "?#"); cut >= 0 {
			rawTarget = rawTarget[:cut]
		}
		source := canonicalDocumentationPath(path.Join(path.Dir(markdownPath), rawTarget))
		file, ok := files[source]
		if !ok {
			warnings = append(warnings, fmt.Sprintf("图片引用缺失：%s", rawTarget))
			return match
		}
		if existing, ok := assetBySource[source]; ok {
			return fmt.Sprintf("![%s](%s)", parts[1], existing)
		}

		config, format, err := image.DecodeConfig(bytes.NewReader(file.Data))
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("图片无法识别，已跳过：%s", rawTarget))
			return match
		}
		if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxDocumentationImagePixels {
			warnings = append(warnings, fmt.Sprintf("图片尺寸超出限制，已跳过：%s", rawTarget))
			return match
		}
		ext := documentationImageFormatExtension(format)
		if ext == "" {
			warnings = append(warnings, fmt.Sprintf("图片格式不受支持，已跳过：%s", rawTarget))
			return match
		}
		assetPath := fmt.Sprintf("assets/%04d%s", len(*assetMeta)+1, ext)
		digest := sha256.Sum256(file.Data)
		assets[assetPath] = file.Data
		assetBySource[source] = assetPath
		*assetMeta = append(*assetMeta, DocumentationAsset{
			Path: assetPath, SHA256: hex.EncodeToString(digest[:]), Bytes: int64(len(file.Data)), Width: config.Width, Height: config.Height,
		})
		return fmt.Sprintf("![%s](%s)", parts[1], assetPath)
	})

	if notionLinkPattern.MatchString(markdown) {
		warnings = append(warnings, "文档中仍有 Notion 链接；目标页面不在本次导出中，链接已原样保留。")
	}
	return markdown, title, warnings, nil
}

func documentationImageFormatExtension(format string) string {
	switch strings.ToLower(format) {
	case "png":
		return ".png"
	case "jpeg":
		return ".jpg"
	case "gif":
		return ".gif"
	case "webp":
		return ".webp"
	default:
		return ""
	}
}

func convertNotionSectionLists(markdown string) string {
	lines := strings.Split(markdown, "\n")
	currentContentIndent := -1
	for index, line := range lines {
		match := sectionBulletPattern.FindStringSubmatch(line)
		if len(match) == 3 {
			indent := len(match[1])
			if indent == 0 || indent == 4 || indent == 8 {
				if hasIndentedNotionContent(lines, index+1, indent) {
					lines[index] = strings.Repeat("#", 2+indent/4) + " " + strings.TrimSpace(match[2])
					currentContentIndent = indent + 4
					continue
				}
			}
		}
		if currentContentIndent > 0 && strings.TrimSpace(line) != "" {
			leading := countLeadingSpaces(line)
			if leading >= currentContentIndent {
				lines[index] = line[currentContentIndent:]
			}
		}
	}
	return strings.Join(lines, "\n")
}

func hasIndentedNotionContent(lines []string, start, indent int) bool {
	for i := start; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		return countLeadingSpaces(lines[i]) > indent
	}
	return false
}

func countLeadingSpaces(line string) int {
	count := 0
	for count < len(line) && line[count] == ' ' {
		count++
	}
	return count
}

func convertNotionAsides(markdown string) string {
	lines := strings.Split(markdown, "\n")
	result := make([]string, 0, len(lines))
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "<aside>" {
			result = append(result, lines[i])
			continue
		}
		body := make([]string, 0)
		closed := false
		for i++; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "</aside>" {
				closed = true
				break
			}
			body = append(body, strings.TrimSpace(lines[i]))
		}
		if !closed {
			result = append(result, "<aside>")
			result = append(result, body...)
			break
		}
		body = trimBlankLines(body)
		if len(body) > 0 && (body[0] == "💡" || body[0] == "⚠️" || body[0] == "ℹ️") {
			body = trimBlankLines(body[1:])
		}
		result = append(result, "> [!TIP]")
		for _, bodyLine := range body {
			if bodyLine == "" {
				result = append(result, ">")
			} else {
				result = append(result, "> "+bodyLine)
			}
		}
	}
	return strings.Join(result, "\n")
}

func trimBlankLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func stripNotionIDsFromHeadings(markdown string) string {
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		match := headingPattern.FindStringSubmatch(line)
		if len(match) == 3 {
			lines[i] = match[1] + " " + notionIDPattern.ReplaceAllString(match[2], "")
		}
	}
	return strings.Join(lines, "\n")
}

func firstDocumentationTitle(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		if strings.HasPrefix(line, "# ") {
			return cleanDocumentationHeadingText(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func demoteDocumentTitle(markdown string) string {
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			lines[i] = "## " + strings.TrimPrefix(line, "# ")
			break
		}
	}
	return strings.Join(lines, "\n")
}

func extractDocumentationOutline(markdown string) []DocumentationHeading {
	result := make([]DocumentationHeading, 0)
	counts := make(map[string]int)
	for _, line := range strings.Split(markdown, "\n") {
		match := headingPattern.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		title := cleanDocumentationHeadingText(match[2])
		base := documentationHeadingID(title)
		counts[base]++
		id := base
		if counts[base] > 1 {
			id = fmt.Sprintf("%s-%d", base, counts[base])
		}
		result = append(result, DocumentationHeading{Level: len(match[1]), Title: title, ID: id})
	}
	return result
}

func cleanDocumentationHeadingText(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "*_` ")
	return value
}

func documentationHeadingID(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= '\u4e00' && r <= '\u9fff') {
			_ = builder.WriteRune(r)
			lastDash = false
		} else if !lastDash && builder.Len() > 0 {
			_ = builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "section"
	}
	return result
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
