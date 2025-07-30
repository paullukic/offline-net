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

	"github.com/Bornholm/go-zim" // Ensure this is imported for http.FS(fs)
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
	BaseZimURL string       // e.g., "/zim/my_zim_name/"
	FileServer http.Handler // The underlying http.FileServer for the ZIM
}

// ServeHTTP handles HTTP requests for a specific ZIM file.
func (zh *ZimFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rw := &responseWriterWrapper{ResponseWriter: w, buf: bytes.NewBuffer(nil)}
	zh.FileServer.ServeHTTP(rw, r) // Serve content into our buffer

	statusCode := rw.statusCode
	originalContentType := rw.Header().Get("Content-Type")
	finalBytes := rw.buf.Bytes()

	// Log the initial request details for debugging
	log.Printf("DEBUG: ZIM '%s' Request: '%s' | Status: %d | Original Content-Type: '%s'",
		zh.ZimName, r.URL.Path, statusCode, originalContentType)

	// Determine the final content type after potential correction
	finalContentType := correctMimeType(r.URL.Path, originalContentType)

	if statusCode == http.StatusOK {
		// If it's HTML, proceed with base tag injection and URL rewriting
		if strings.HasPrefix(finalContentType, "text/html") {
			// Calculate the base directory for resolving relative URLs within the ZIM
			currentDirForRelativeResolution := resolveCurrentDir(zh.BaseZimURL, r.URL.Path)

			log.Printf("DEBUG: ZIM '%s' HTML Processing: ZIM Base URL: '%s', Current Dir for Relative Resolution: '%s'",
				zh.ZimName, zh.BaseZimURL, currentDirForRelativeResolution)

			modified, err := injectBaseTagAndRewritePaths(
				finalBytes,
				zh.BaseZimURL,
				currentDirForRelativeResolution,
			)
			if err != nil {
				log.Printf("WARNING: Failed to modify HTML for ZIM '%s' Path: '%s': %v. Serving original HTML.",
					zh.ZimName, r.URL.Path, err)
			} else {
				finalBytes = modified
				log.Printf("DEBUG: ZIM '%s' Path: '%s' | Successfully modified HTML (base tag & all relative paths rewritten).", zh.ZimName, r.URL.Path)
			}
			finalContentType = "text/html; charset=utf-8" // Ensure explicit charset for HTML
		}
	}

	// Copy all original headers except Content-Type and Content-Length
	copyHeaders(w.Header(), rw.Header())
	// Set the final (potentially corrected) Content-Type and Content-Length
	w.Header().Set("Content-Type", finalContentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(finalBytes)))
	// Write the captured status code
	w.WriteHeader(statusCode)
	// Write the final (potentially modified) bytes
	w.Write(finalBytes)
}

// responseWriterWrapper is a helper to capture the HTTP response body and status.
type responseWriterWrapper struct {
	http.ResponseWriter
	buf        *bytes.Buffer
	statusCode int // Captured status code
}

// Header returns the header map of the underlying ResponseWriter.
func (rw *responseWriterWrapper) Header() http.Header {
	return rw.ResponseWriter.Header()
}

// Write captures the body content into an internal buffer.
func (rw *responseWriterWrapper) Write(b []byte) (int, error) {
	return rw.buf.Write(b)
}

// WriteHeader captures the status code. It does NOT call the underlying ResponseWriter's WriteHeader directly,
// allowing the outer handler to set all headers and the status code at the end.
func (rw *responseWriterWrapper) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
}

// correctMimeType attempts to fix common MIME type issues for assets.
func correctMimeType(path, originalCT string) string {
	pathLower := strings.ToLower(path)
	// Check if the original content type is "text/plain" or empty, or if it doesn't match the expected prefix.
	// This makes the correction more robust.
	switch {
	case strings.HasSuffix(pathLower, ".css") && (originalCT == "text/plain" || originalCT == "" || !strings.HasPrefix(originalCT, "text/css")):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(pathLower, ".js") && (originalCT == "text/plain" || originalCT == "" || !strings.HasPrefix(originalCT, "application/javascript")):
		return "application/javascript; charset=utf-8"
	case strings.HasSuffix(pathLower, ".png") && (originalCT == "text/plain" || originalCT == "" || !strings.HasPrefix(originalCT, "image/png")):
		return "image/png"
	case (strings.HasSuffix(pathLower, ".jpg") || strings.HasSuffix(pathLower, ".jpeg")) && (originalCT == "text/plain" || originalCT == "" || !strings.HasPrefix(originalCT, "image/jpeg")):
		return "image/jpeg"
	case strings.HasSuffix(pathLower, ".gif") && (originalCT == "text/plain" || originalCT == "" || !strings.HasPrefix(originalCT, "image/gif")):
		return "image/gif"
	default:
		return originalCT // Return original if no specific correction is needed
	}
}

// copyHeaders copies all headers from source to destination, excluding specified ones.
func copyHeaders(dst, src http.Header) {
	for k, v := range src {
		// Exclude Content-Type and Content-Length as they are set explicitly later
		if strings.ToLower(k) == "content-type" || strings.ToLower(k) == "content-length" {
			continue
		}
		for _, vv := range v {
			dst.Add(k, vv)
		}
	}
}

// resolveCurrentDir returns the directory URL for the current request path within the ZIM's URL space.
// E.g., base="/zim/myzim/", path="/category/page.html" -> "/zim/myzim/category/"
func resolveCurrentDir(baseZimURL, requestPath string) string {
	if requestPath == "/" || requestPath == "" {
		return baseZimURL // If it's the ZIM's root, the base is the ZIM's URL
	}
	// filepath.Dir gets the directory part of the path (e.g., "/category" from "/category/page.html")
	dir := filepath.Dir(requestPath)
	// Ensure the directory path ends with a slash for correct URL resolution
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	return baseZimURL + strings.TrimPrefix(dir, "/") // Combine ZIM base with the internal directory path
}

// resolveCurrentRelativeURL resolves a relative URL against a given base path.
// This is used for paths like "image.png" or "../../style.css".
func resolveCurrentRelativeURL(basePathForResolution, relativeURL string) (string, error) {
	// A dummy scheme and host are needed for url.Parse to correctly handle all forms of relative paths.
	// We'll strip them later. basePathForResolution should already end with a slash.
	dummyBaseURL := "http://dummyhost" + basePathForResolution
	base, err := url.Parse(dummyBaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL '%s': %w", dummyBaseURL, err)
	}
	rel, err := url.Parse(relativeURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse relative URL '%s': %w", relativeURL, err)
	}
	resolvedURL := base.ResolveReference(rel)
	// Return only the path component, stripping the dummy scheme and host.
	return resolvedURL.Path, nil
}

// injectBaseTagAndRewritePaths parses HTML, injects a <base> tag, and rewrites resource URLs.
func injectBaseTagAndRewritePaths(htmlBytes []byte, zimBaseURL, currentDirForRelativeResolution string) ([]byte, error) {
	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var baseTagInserted bool // Flag to ensure <base> tag is inserted only once

	var f func(*html.Node) // Recursive function to traverse HTML nodes
	f = func(n *html.Node) {
		if n.Type == html.ElementNode {
			// Handle <base> tag insertion in the <head>
			if n.Data == "head" && !baseTagInserted {
				removeBaseTags(n) // Remove any existing base tags
				baseTag := &html.Node{
					Type: html.ElementNode,
					Data: "base",
					Attr: []html.Attribute{{Key: "href", Val: zimBaseURL}}, // Set href to the ZIM's root URL
				}
				n.InsertBefore(baseTag, n.FirstChild) // Insert as first child
				baseTagInserted = true
				log.Printf("DEBUG: Injected <base href='%s'> in HTML head.", zimBaseURL)
			}

			// Rewrite href and src attributes
			for i, a := range n.Attr {
				// Only process href/src that are not already full absolute URLs, anchors, or mailto links
				if (a.Key == "href" || a.Key == "src") &&
					!strings.HasPrefix(a.Val, "http://") &&
					!strings.HasPrefix(a.Val, "https://") &&
					!strings.HasPrefix(a.Val, "#") &&
					!strings.HasPrefix(a.Val, "./") &&
					!strings.HasPrefix(a.Val, "/") &&
					!strings.HasPrefix(a.Val, "mailto:") {

					originalVal := a.Val
					rewrittenVal := rewriteURL(originalVal, zimBaseURL, currentDirForRelativeResolution)
					if rewrittenVal != originalVal { // Only log if a change occurred
						log.Printf("DEBUG: Rewrote link: '%s' -> '%s'", originalVal, rewrittenVal)
					}
					n.Attr[i].Val = rewrittenVal
				}
			}
		}
		// Recurse for child nodes
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc) // Start traversal from the document root

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return nil, fmt.Errorf("failed to render HTML: %w", err)
	}
	return buf.Bytes(), nil
}

// removeBaseTags removes any existing <base> tags from the <head> node.
func removeBaseTags(head *html.Node) {
	for c := head.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.ElementNode && c.Data == "base" {
			head.RemoveChild(c)
		}
		c = next
	}
}

// rewriteURL determines the correct absolute URL for a given relative path.
func rewriteURL(val, zimBaseURL, currentDir string) string {
	// If the value starts with '/', it's a root-relative path within the ZIM's context.
	// We need to prepend the full ZIM base URL.
	if strings.HasPrefix(val, "/") {
		// Avoid double-prepending if it already has the full ZIM base URL
		if strings.HasPrefix(val, zimBaseURL) {
			return val
		}
		return zimBaseURL + strings.TrimPrefix(val, "/")
	}

	// If it's a truly relative path (e.g., "image.png", "../style.css"),
	// resolve it against the current directory.
	resolved, err := resolveCurrentRelativeURL(currentDir, val)
	if err != nil {
		log.Printf("WARNING: Error resolving relative URL '%s' against '%s': %v. Returning original.", val, currentDir, err)
		return val // Return original if resolution fails
	}

	// After resolving, ensure the path is correctly prefixed with the ZIM base URL.
	// This handles cases where `resolveCurrentRelativeURL` might yield `/zim/something`
	// but not yet `/zim/myzim/something`.
	if strings.HasPrefix(resolved, zimBaseURL) {
		return resolved
	}
	if strings.HasPrefix(resolved, "/zim/") {
		// If it resolved to /zim/ but not /zim/myzim/, fix it.
		return zimBaseURL + strings.TrimPrefix(resolved, "/zim/")
	}
	// Fallback for other cases, prepend zimBaseURL
	return zimBaseURL + strings.TrimPrefix(resolved, "/")
}

// zimMetadataCache stores metadata for loaded ZIM files, used by the dashboard.
var zimMetadataCache = make(map[string]ZimEntry)

// SetupZimFiles scans the zimFilesDir, opens each ZIM, and stores its reader and metadata.
func SetupZimFiles(serverPort, zimFilesDir, zimContentBasePath string) (map[string]*zim.Reader, error) {
	zimReaders := make(map[string]*zim.Reader)
	zimMetadataCache = make(map[string]ZimEntry) // Clear cache on setup

	files, err := os.ReadDir(zimFilesDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: ZIM files directory '%s' does not exist. No ZIM files will be served.", zimFilesDir)
			return zimReaders, nil // No error if directory just doesn't exist
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

// ListZimEntriesForDashboard returns the list of loaded ZIM entries for the dashboard.
func ListZimEntriesForDashboard() []ZimEntry {
	entries := make([]ZimEntry, 0, len(zimMetadataCache)) // Pre-allocate slice for efficiency
	for _, entry := range zimMetadataCache {
		entries = append(entries, entry)
	}
	return entries
}
