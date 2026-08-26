package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

var notionHTMLImageWidthPattern = regexp.MustCompile(`(?i)(?:^|;)\s*width\s*:\s*([0-9]+(?:\.[0-9]+)?)px`)

type notionHTMLDocument struct {
	path      string
	title     string
	rootID    string
	article   *html.Node
	pageTitle *html.Node
}

func importNotionHTMLDocuments(files map[string]documentationArchiveFile, htmlPaths []string, initialWarnings []string) (*documentationImportResult, error) {
	assets := make(map[string][]byte)
	assetBySource := make(map[string]string)
	assetMeta := make([]DocumentationAsset, 0)
	warnings := append([]string(nil), initialWarnings...)
	outline := make([]DocumentationHeading, 0)
	idCounts := make(map[string]int)
	documents := make([]*notionHTMLDocument, 0, len(htmlPaths))
	documentByPath := make(map[string]*notionHTMLDocument, len(htmlPaths))
	fragmentIDs := make(map[string]string)

	for _, htmlPath := range htmlPaths {
		file := files[htmlPath]
		if len(file.Data) > maxDocumentationContentBytes {
			return nil, fmt.Errorf("%w: HTML file %q exceeds %d bytes", errDocumentationArchiveTooLarge, htmlPath, maxDocumentationContentBytes)
		}
		if !utf8.Valid(file.Data) {
			return nil, fmt.Errorf("%w: HTML file %q is not UTF-8", errDocumentationInvalidArchive, htmlPath)
		}
		parsed, err := html.Parse(bytes.NewReader(file.Data))
		if err != nil {
			return nil, fmt.Errorf("%w: cannot parse HTML file %q: %v", errDocumentationInvalidArchive, htmlPath, err)
		}
		article := findNotionHTMLElement(parsed, func(node *html.Node) bool { return node.Data == "article" })
		if article == nil {
			article = findNotionHTMLElement(parsed, func(node *html.Node) bool { return node.Data == "body" })
		}
		if article == nil {
			return nil, fmt.Errorf("%w: HTML file %q does not contain a document body", errDocumentationInvalidArchive, htmlPath)
		}
		pageTitle := findNotionHTMLElement(article, func(node *html.Node) bool {
			return node.Data == "h1" && notionHTMLHasClass(node, "page-title")
		})
		title := notionHTMLNodeText(pageTitle)
		if title == "" {
			titleNode := findNotionHTMLElement(parsed, func(node *html.Node) bool { return node.Data == "title" })
			title = notionHTMLNodeText(titleNode)
		}
		if title == "" {
			title = notionIDPattern.ReplaceAllString(strings.TrimSuffix(path.Base(htmlPath), path.Ext(htmlPath)), "")
		}
		title = cleanDocumentationHeadingText(title)
		rootID := uniqueDocumentationHeadingID(title, idCounts)
		document := &notionHTMLDocument{path: htmlPath, title: title, rootID: rootID, article: article, pageTitle: pageTitle}
		documents = append(documents, document)
		documentByPath[htmlPath] = document
	}

	for index, document := range documents {
		sanitizeNotionHTMLTree(document.article, &warnings)
		rewriteNotionHTMLImages(document.article, document.path, files, assetBySource, assets, &assetMeta, &warnings)

		pageLevel := 1
		if index > 0 {
			pageLevel = 2
		}
		if document.pageTitle != nil {
			oldID := notionHTMLAttribute(document.pageTitle, "id")
			setNotionHTMLAttribute(document.pageTitle, "id", document.rootID)
			addNotionHTMLClass(document.pageTitle, "docs-page-title")
			if oldID != "" {
				fragmentIDs[document.path+"#"+oldID] = document.rootID
			}
		}
		if oldID := notionHTMLAttribute(document.article, "id"); oldID != "" {
			fragmentIDs[document.path+"#"+oldID] = document.rootID
		}
		outline = append(outline, DocumentationHeading{Level: pageLevel, Title: document.title, ID: document.rootID})
		assignNotionHTMLSections(document.article, document, pageLevel, 0, idCounts, fragmentIDs, &outline)
	}

	for _, document := range documents {
		rewriteNotionHTMLLinks(document.article, document, files, documentByPath, fragmentIDs, assetBySource, assets, &assetMeta, &warnings)
	}

	var output strings.Builder
	for index, document := range documents {
		if index > 0 {
			_, _ = output.WriteString(`<hr class="docs-document-divider">`)
		}
		_, _ = output.WriteString(`<section class="notion-document" data-docs-document="`)
		_, _ = output.WriteString(html.EscapeString(document.rootID))
		_, _ = output.WriteString(`">`)
		for child := document.article.FirstChild; child != nil; child = child.NextSibling {
			if err := html.Render(&output, child); err != nil {
				return nil, fmt.Errorf("%w: cannot serialize HTML file %q: %v", errDocumentationInvalidArchive, document.path, err)
			}
		}
		_, _ = output.WriteString(`</section>`)
	}
	content := []byte(output.String())
	if len(content) == 0 || len(content) > maxDocumentationContentBytes {
		return nil, fmt.Errorf("%w: normalized HTML exceeds %d bytes", errDocumentationArchiveTooLarge, maxDocumentationContentBytes)
	}

	warnings = uniqueStrings(warnings)
	sort.Strings(warnings)
	return &documentationImportResult{
		Title:     documents[0].title,
		Format:    documentationFormatHTML,
		Content:   content,
		Assets:    assets,
		AssetMeta: assetMeta,
		Outline:   outline,
		Warnings:  warnings,
	}, nil
}

func sanitizeNotionHTMLTree(parent *html.Node, warnings *[]string) {
	for child := parent.FirstChild; child != nil; {
		next := child.NextSibling
		if child.Type == html.ElementNode && isUnsafeNotionHTMLElement(child.Data) {
			*warnings = append(*warnings, fmt.Sprintf("已移除不安全或不支持的 HTML 元素：%s", child.Data))
			parent.RemoveChild(child)
			child = next
			continue
		}
		if child.Type == html.CommentNode {
			parent.RemoveChild(child)
			child = next
			continue
		}
		if child.Type == html.ElementNode {
			if child.Data == "img" {
				if match := notionHTMLImageWidthPattern.FindStringSubmatch(notionHTMLAttribute(child, "style")); len(match) == 2 {
					if width, err := strconv.ParseFloat(match[1], 64); err == nil && width > 0 && width <= 10000 {
						setNotionHTMLAttribute(child, "data-docs-width", strconv.Itoa(int(width+0.5)))
					}
				}
			}
			attrs := child.Attr[:0]
			for _, attr := range child.Attr {
				name := strings.ToLower(attr.Key)
				if strings.HasPrefix(name, "on") || name == "style" || name == "srcset" || name == "sizes" || name == "srcdoc" || name == "nonce" || name == "integrity" {
					continue
				}
				attrs = append(attrs, attr)
			}
			child.Attr = attrs
			if child.Data == "input" {
				setNotionHTMLAttribute(child, "disabled", "")
			}
		}
		sanitizeNotionHTMLTree(child, warnings)
		child = next
	}
}

func isUnsafeNotionHTMLElement(name string) bool {
	switch strings.ToLower(name) {
	case "script", "style", "link", "meta", "base", "iframe", "object", "embed", "form", "template":
		return true
	default:
		return false
	}
}

func rewriteNotionHTMLImages(root *html.Node, htmlPath string, files map[string]documentationArchiveFile, assetBySource map[string]string, assets map[string][]byte, assetMeta *[]DocumentationAsset, warnings *[]string) {
	walkNotionHTML(root, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "img" {
			return
		}
		rawTarget := strings.TrimSpace(notionHTMLAttribute(node, "src"))
		if rawTarget == "" || strings.HasPrefix(rawTarget, "data:") {
			return
		}
		parsed, err := url.Parse(rawTarget)
		if err != nil || parsed.Scheme != "" || parsed.Host != "" || strings.HasPrefix(rawTarget, "//") {
			*warnings = append(*warnings, fmt.Sprintf("远程图片已原样保留：%s", rawTarget))
			return
		}
		source, ok := resolveNotionHTMLArchiveTarget(htmlPath, parsed.Path)
		if !ok {
			*warnings = append(*warnings, fmt.Sprintf("图片引用无法解析：%s", rawTarget))
			return
		}
		assetPath, err := registerNotionHTMLImage(source, files, assetBySource, assets, assetMeta)
		if err != nil {
			*warnings = append(*warnings, fmt.Sprintf("图片无法导入：%s（%v）", rawTarget, err))
			return
		}
		setNotionHTMLAttribute(node, "src", assetPath)
	})
}

func registerNotionHTMLImage(source string, files map[string]documentationArchiveFile, assetBySource map[string]string, assets map[string][]byte, assetMeta *[]DocumentationAsset) (string, error) {
	if existing, ok := assetBySource[source]; ok {
		return existing, nil
	}
	file, ok := files[source]
	if !ok {
		return "", fmt.Errorf("文件不存在")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(file.Data))
	if err != nil {
		return "", fmt.Errorf("无法识别图片")
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxDocumentationImagePixels {
		return "", fmt.Errorf("图片尺寸超出限制")
	}
	ext := documentationImageFormatExtension(format)
	if ext == "" {
		return "", fmt.Errorf("图片格式不受支持")
	}
	assetPath := fmt.Sprintf("assets/%04d%s", len(*assetMeta)+1, ext)
	digest := sha256.Sum256(file.Data)
	assets[assetPath] = file.Data
	assetBySource[source] = assetPath
	*assetMeta = append(*assetMeta, DocumentationAsset{
		Path: assetPath, SHA256: hex.EncodeToString(digest[:]), Bytes: int64(len(file.Data)), Width: config.Width, Height: config.Height,
	})
	return assetPath, nil
}

func assignNotionHTMLSections(root *html.Node, document *notionHTMLDocument, pageLevel, detailsDepth int, idCounts map[string]int, fragmentIDs map[string]string, outline *[]DocumentationHeading) {
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			assignNotionHTMLSections(child, document, pageLevel, detailsDepth, idCounts, fragmentIDs, outline)
			continue
		}
		if child.Data == "details" {
			summary := directNotionHTMLChild(child, "summary")
			title := cleanDocumentationHeadingText(notionHTMLNodeText(summary))
			if title != "" {
				level := pageLevel + detailsDepth + 1
				if level > 4 {
					level = 4
				}
				id := uniqueDocumentationHeadingID(title, idCounts)
				oldID := notionHTMLAttribute(child, "id")
				setNotionHTMLAttribute(child, "id", id)
				setNotionHTMLAttribute(child, "data-docs-section", "")
				setNotionHTMLAttribute(child, "data-docs-level", strconv.Itoa(level))
				addNotionHTMLClass(child, "docs-toggle")
				addNotionHTMLClass(child, fmt.Sprintf("docs-toggle-level-%d", level))
				if summary != nil {
					addNotionHTMLClass(summary, "docs-toggle-summary")
				}
				if oldID != "" {
					fragmentIDs[document.path+"#"+oldID] = id
				}
				*outline = append(*outline, DocumentationHeading{Level: level, Title: title, ID: id})
			}
			assignNotionHTMLSections(child, document, pageLevel, detailsDepth+1, idCounts, fragmentIDs, outline)
			continue
		}
		if isNotionHTMLHeading(child.Data) && child != document.pageTitle && !hasNotionHTMLAncestor(child, "summary") {
			title := cleanDocumentationHeadingText(notionHTMLNodeText(child))
			if title != "" {
				level := int(child.Data[1] - '0')
				if level == 1 {
					level = 2
				}
				id := uniqueDocumentationHeadingID(title, idCounts)
				oldID := notionHTMLAttribute(child, "id")
				setNotionHTMLAttribute(child, "id", id)
				if oldID != "" {
					fragmentIDs[document.path+"#"+oldID] = id
				}
				*outline = append(*outline, DocumentationHeading{Level: level, Title: title, ID: id})
			}
		}
		assignNotionHTMLSections(child, document, pageLevel, detailsDepth, idCounts, fragmentIDs, outline)
	}
}

func rewriteNotionHTMLLinks(root *html.Node, document *notionHTMLDocument, files map[string]documentationArchiveFile, documentByPath map[string]*notionHTMLDocument, fragmentIDs map[string]string, assetBySource map[string]string, assets map[string][]byte, assetMeta *[]DocumentationAsset, warnings *[]string) {
	walkNotionHTML(root, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "a" {
			return
		}
		rawTarget := strings.TrimSpace(notionHTMLAttribute(node, "href"))
		if rawTarget == "" {
			return
		}
		parsed, err := url.Parse(rawTarget)
		if err != nil {
			removeNotionHTMLAttribute(node, "href")
			return
		}
		if parsed.Scheme != "" || parsed.Host != "" || strings.HasPrefix(rawTarget, "//") {
			if !isSafeDocumentationLinkScheme(parsed.Scheme) {
				removeNotionHTMLAttribute(node, "href")
				*warnings = append(*warnings, fmt.Sprintf("已移除不安全链接：%s", rawTarget))
			}
			return
		}

		targetPath := document.path
		if parsed.Path != "" {
			resolved, ok := resolveNotionHTMLArchiveTarget(document.path, parsed.Path)
			if !ok {
				removeNotionHTMLAttribute(node, "href")
				return
			}
			targetPath = resolved
		}
		ext := strings.ToLower(path.Ext(targetPath))
		switch {
		case ext == ".html" || ext == ".htm":
			targetDocument, ok := documentByPath[targetPath]
			if !ok {
				removeNotionHTMLAttribute(node, "href")
				*warnings = append(*warnings, fmt.Sprintf("站内页面链接目标缺失：%s", rawTarget))
				return
			}
			targetID := targetDocument.rootID
			if parsed.Fragment != "" {
				fragment, unescapeErr := url.PathUnescape(parsed.Fragment)
				if unescapeErr == nil {
					if mapped, exists := fragmentIDs[targetPath+"#"+fragment]; exists {
						targetID = mapped
					}
				}
			}
			setNotionHTMLAttribute(node, "href", "#"+targetID)
		case parsed.Path == "" && parsed.Fragment != "":
			fragment, unescapeErr := url.PathUnescape(parsed.Fragment)
			if unescapeErr == nil {
				if mapped, exists := fragmentIDs[document.path+"#"+fragment]; exists {
					setNotionHTMLAttribute(node, "href", "#"+mapped)
				}
			}
		case isDocumentationImageExtension(ext):
			assetPath, assetErr := registerNotionHTMLImage(targetPath, files, assetBySource, assets, assetMeta)
			if assetErr != nil {
				removeNotionHTMLAttribute(node, "href")
				return
			}
			setNotionHTMLAttribute(node, "href", assetPath)
		default:
			removeNotionHTMLAttribute(node, "href")
			*warnings = append(*warnings, fmt.Sprintf("已移除无法发布的本地链接：%s", rawTarget))
		}
	})
}

func resolveNotionHTMLArchiveTarget(documentPath, rawTarget string) (string, bool) {
	decoded, err := url.PathUnescape(strings.TrimSpace(rawTarget))
	if err != nil || decoded == "" || strings.Contains(decoded, "\\") || strings.HasPrefix(decoded, "/") {
		return "", false
	}
	resolved := canonicalDocumentationPath(path.Join(path.Dir(documentPath), decoded))
	if resolved == "." || resolved == ".." || strings.HasPrefix(resolved, "../") {
		return "", false
	}
	return resolved, true
}

func isSafeDocumentationLinkScheme(scheme string) bool {
	switch strings.ToLower(scheme) {
	case "http", "https", "mailto", "tel":
		return true
	default:
		return false
	}
}

func uniqueDocumentationHeadingID(title string, counts map[string]int) string {
	base := documentationHeadingID(title)
	counts[base]++
	if counts[base] == 1 {
		return base
	}
	return fmt.Sprintf("%s-%d", base, counts[base])
}

func findNotionHTMLElement(root *html.Node, predicate func(*html.Node) bool) *html.Node {
	if root == nil {
		return nil
	}
	if root.Type == html.ElementNode && predicate(root) {
		return root
	}
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if found := findNotionHTMLElement(child, predicate); found != nil {
			return found
		}
	}
	return nil
}

func walkNotionHTML(root *html.Node, visit func(*html.Node)) {
	if root == nil {
		return
	}
	visit(root)
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		walkNotionHTML(child, visit)
	}
}

func directNotionHTMLChild(root *html.Node, name string) *html.Node {
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == name {
			return child
		}
	}
	return nil
}

func notionHTMLNodeText(root *html.Node) string {
	if root == nil {
		return ""
	}
	var builder strings.Builder
	walkNotionHTML(root, func(node *html.Node) {
		if node.Type == html.TextNode {
			_, _ = builder.WriteString(node.Data)
		}
	})
	return strings.Join(strings.Fields(builder.String()), " ")
}

func notionHTMLHasClass(node *html.Node, className string) bool {
	for _, value := range strings.Fields(notionHTMLAttribute(node, "class")) {
		if value == className {
			return true
		}
	}
	return false
}

func addNotionHTMLClass(node *html.Node, className string) {
	if node == nil || notionHTMLHasClass(node, className) {
		return
	}
	classes := strings.TrimSpace(notionHTMLAttribute(node, "class") + " " + className)
	setNotionHTMLAttribute(node, "class", classes)
}

func notionHTMLAttribute(node *html.Node, name string) string {
	if node == nil {
		return ""
	}
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, name) {
			return attr.Val
		}
	}
	return ""
}

func setNotionHTMLAttribute(node *html.Node, name, value string) {
	if node == nil {
		return
	}
	for index := range node.Attr {
		if strings.EqualFold(node.Attr[index].Key, name) {
			node.Attr[index].Key = name
			node.Attr[index].Val = value
			return
		}
	}
	node.Attr = append(node.Attr, html.Attribute{Key: name, Val: value})
}

func removeNotionHTMLAttribute(node *html.Node, name string) {
	if node == nil {
		return
	}
	attrs := node.Attr[:0]
	for _, attr := range node.Attr {
		if !strings.EqualFold(attr.Key, name) {
			attrs = append(attrs, attr)
		}
	}
	node.Attr = attrs
}

func hasNotionHTMLAncestor(node *html.Node, name string) bool {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if parent.Type == html.ElementNode && parent.Data == name {
			return true
		}
	}
	return false
}

func isNotionHTMLHeading(name string) bool {
	return len(name) == 2 && name[0] == 'h' && name[1] >= '1' && name[1] <= '4'
}
