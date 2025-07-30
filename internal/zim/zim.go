package zim

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Bornholm/go-zim"
	"golang.org/x/net/html"
)

// ZimEntry represents a ZIM file for the dashboard.
type ZimEntry struct {
	FileName    string
	Title       string
	Description string
	AccessURL   string
	ZimReader   *zim.Reader
}

// ZimFileHandler wraps a go-zim file server and rewrites HTML responses.
type ZimFileHandler struct {
	ZimName    string
	BaseZimURL string
	FileServer http.Handler
}

func (zh *ZimFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rw := &responseWriterWrapper{ResponseWriter: w, buf: bytes.NewBuffer(nil)}
	zh.FileServer.ServeHTTP(rw, r)

	statusCode := rw.statusCode
	contentType := rw.Header().Get("Content-Type")
	finalBytes := rw.buf.Bytes()

	if statusCode == http.StatusOK {
		contentType = correctMimeType(r.URL.Path, contentType)
		if strings.HasPrefix(contentType, "text/html") {
			modified, err := injectBaseTagAndRewritePaths(
				finalBytes,
				zh.BaseZimURL,
				resolveCurrentDir(zh.BaseZimURL, r.URL.Path),
			)
			if err == nil {
				finalBytes = modified
			}
			contentType = "text/html; charset=utf-8"
		}
	}

	copyHeaders(w.Header(), rw.Header())
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(finalBytes)))
	w.WriteHeader(statusCode)
	w.Write(finalBytes)
}

type responseWriterWrapper struct {
	http.ResponseWriter
	buf        *bytes.Buffer
	statusCode int
}

func (rw *responseWriterWrapper) Header() http.Header         { return rw.ResponseWriter.Header() }
func (rw *responseWriterWrapper) Write(b []byte) (int, error) { return rw.buf.Write(b) }
func (rw *responseWriterWrapper) WriteHeader(statusCode int)  { rw.statusCode = statusCode }

func correctMimeType(path, ct string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".css") && (ct == "text/plain" || ct == ""):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(lower, ".js") && (ct == "text/plain" || ct == ""):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(lower, ".png") && (ct == "text/plain" || ct == ""):
		return "image/png"
	case (strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg")) && (ct == "text/plain" || ct == ""):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".gif") && (ct == "text/plain" || ct == ""):
		return "image/gif"
	default:
		return ct
	}
}

func copyHeaders(dst, src http.Header) {
	for k, v := range src {
		if strings.ToLower(k) == "content-type" || strings.ToLower(k) == "content-length" {
			continue
		}
		for _, vv := range v {
			dst.Add(k, vv)
		}
	}
}

// resolveCurrentDir returns the directory URL for the current request path.
func resolveCurrentDir(base, path string) string {
	if path == "/" || path == "" {
		return base
	}
	dir := filepath.Dir(path)
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	return base + dir
}

// resolveCurrentRelativeURL resolves a relative URL against the current path.
func resolveCurrentRelativeURL(currentFullPath, relativeURL string) (string, error) {
	dummyBaseURL := "http://dummyhost" + currentFullPath
	base, err := url.Parse(dummyBaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL '%s': %w", dummyBaseURL, err)
	}
	rel, err := url.Parse(relativeURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse relative URL '%s': %w", relativeURL, err)
	}
	return base.ResolveReference(rel).Path, nil
}

// injectBaseTagAndRewritePaths parses HTML, injects a <base> tag, and rewrites resource URLs.
func injectBaseTagAndRewritePaths(htmlBytes []byte, zimBaseURL, currentDir string) ([]byte, error) {
	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	var baseTagInserted bool
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "head" && !baseTagInserted {
				removeBaseTags(n)
				baseTag := &html.Node{
					Type: html.ElementNode,
					Data: "base",
					Attr: []html.Attribute{{Key: "href", Val: zimBaseURL}},
				}
				n.InsertBefore(baseTag, n.FirstChild)
				baseTagInserted = true
			}
			for i, a := range n.Attr {
				if (a.Key == "href" || a.Key == "src") &&
					!strings.HasPrefix(a.Val, "http://") &&
					!strings.HasPrefix(a.Val, "https://") &&
					!strings.HasPrefix(a.Val, "#") &&
					!strings.HasPrefix(a.Val, "mailto:") {
					n.Attr[i].Val = rewriteURL(a.Val, zimBaseURL, currentDir)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return nil, fmt.Errorf("failed to render HTML: %w", err)
	}
	return buf.Bytes(), nil
}

func removeBaseTags(head *html.Node) {
	for c := head.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.ElementNode && c.Data == "base" {
			head.RemoveChild(c)
		}
		c = next
	}
}

func rewriteURL(val, zimBaseURL, currentDir string) string {
	if strings.HasPrefix(val, "/") {
		if strings.HasPrefix(val, zimBaseURL) {
			return val
		}
		return zimBaseURL + strings.TrimPrefix(val, "/")
	}
	resolved, err := resolveCurrentRelativeURL(currentDir, val)
	if err != nil {
		return val
	}
	if strings.HasPrefix(resolved, zimBaseURL) {
		return resolved
	}
	if strings.HasPrefix(resolved, "/zim/") {
		return zimBaseURL + strings.TrimPrefix(resolved, "/zim/")
	}
	return zimBaseURL + strings.TrimPrefix(resolved, "/")
}

var zimMetadataCache = make(map[string]ZimEntry)

func SetupZimFiles(serverPort, zimFilesDir, zimContentBasePath string) (map[string]*zim.Reader, error) {
	zimReaders := make(map[string]*zim.Reader)
	zimMetadataCache = make(map[string]ZimEntry)

	files, err := os.ReadDir(zimFilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: ZIM files directory '%s' does not exist.", zimFilesDir)
			return zimReaders, nil
		}
		return nil, fmt.Errorf("failed to read ZIM files directory '%s': %w", zimFilesDir, err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".zim") {
			zimFilePath := filepath.Join(zimFilesDir, file.Name())
			zimBaseName := strings.TrimSuffix(file.Name(), ".zim")
			reader, err := zim.Open(zimFilePath)
			if err != nil {
				log.Printf("WARNING: Failed to open ZIM file '%s': %v", file.Name(), err)
				continue
			}
			zimReaders[zimBaseName] = reader

			title, description := file.Name(), "Offline ZIM content"
			if metadata, _ := reader.Metadata(); metadata != nil {
				if desc, ok := metadata[zim.MetadataKey("Description")]; ok && desc != "" {
					description = desc
				}
			}
			if mainPage, err := reader.MainPage(); err == nil {
				if t := mainPage.Title(); t != "" {
					title = t
				}
			}
			zimMetadataCache[zimBaseName] = ZimEntry{
				FileName:    file.Name(),
				Title:       title,
				Description: description,
				AccessURL:   fmt.Sprintf("http://localhost%s%s%s/", serverPort, zimContentBasePath, zimBaseName),
				ZimReader:   reader,
			}
			log.Printf("Loaded ZIM '%s' for serving at %s", file.Name(), zimMetadataCache[zimBaseName].AccessURL)
		}
	}
	return zimReaders, nil
}

func ListZimEntriesForDashboard() []ZimEntry {
	entries := make([]ZimEntry, 0, len(zimMetadataCache))
	for _, entry := range zimMetadataCache {
		entries = append(entries, entry)
	}
	return entries
}
