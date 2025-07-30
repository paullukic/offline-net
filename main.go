package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"offline-net/internal/zim"

	"github.com/Bornholm/go-zim/fs"
)

const (
	serverPort         = ":8000"
	zimFilesDir        = "./zim-content"
	staticFilesDir     = "./web-content"
	templatesDir       = "./templates"
	zimContentBasePath = "/zim/"
)

type LibraryPageData struct {
	ZimEntries     []zim.ZimEntry
	WebSites       []WebSite
	ServerURL      string
	ZimFilesDir    string
	StaticFilesDir string
}

type WebSite struct {
	Name      string
	AccessURL string
}

func main() {
	log.Println("Starting Offline Knowledge & AI Server...")

	registerStaticFileServer()
	registerZimHandlers()
	registerCustomSitesHandler()
	registerLibraryHandler()

	addr := fmt.Sprintf("http://localhost%s/", serverPort)
	log.Printf("Serving library dashboard at %s", addr)
	fmt.Printf("Server is starting on %s. Access it in your browser at %s\n", serverPort, addr)
	log.Fatal(http.ListenAndServe(serverPort, nil))
}

func registerStaticFileServer() {
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
}

func registerZimHandlers() {
	zimReaders, err := zim.SetupZimFiles(serverPort, zimFilesDir, zimContentBasePath)
	if err != nil {
		log.Fatalf("Fatal: Could not setup ZIM files for serving: %v", err)
	}
	for zimName, reader := range zimReaders {
		urlPath := fmt.Sprintf("%s%s/", zimContentBasePath, zimName)
		zimFs := fs.New(reader)
		rawFileServer := http.FileServer(http.FS(zimFs))
		handler := &zim.ZimFileHandler{
			ZimName:    zimName,
			BaseZimURL: urlPath,
			FileServer: rawFileServer,
		}
		http.Handle(urlPath, http.StripPrefix(urlPath, handler))
		log.Printf("Registered ZIM handler for '%s' at %s", zimName, urlPath)
	}
}

func registerCustomSitesHandler() {
	http.Handle("/custom-site/", http.StripPrefix("/custom-site/", http.FileServer(http.Dir(staticFilesDir))))
	log.Printf("Serving custom static content from '%s' at http://localhost%s/custom-site/", staticFilesDir, serverPort)
}

func registerLibraryHandler() {
	http.HandleFunc("/", libraryHandler)
}

func libraryHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tmpl, err := template.ParseFiles(filepath.Join(templatesDir, "library.html"))
	if err != nil {
		log.Printf("ERROR: Failed to load library template: %v", err)
		http.Error(w, "Failed to load library page.", http.StatusInternalServerError)
		return
	}
	data := LibraryPageData{
		ServerURL:      fmt.Sprintf("http://localhost%s", serverPort),
		ZimFilesDir:    zimFilesDir,
		StaticFilesDir: staticFilesDir,
		ZimEntries:     zim.ListZimEntriesForDashboard(),
	}
	if sites, err := os.ReadDir(staticFilesDir); err == nil {
		for _, dirEntry := range sites {
			if dirEntry.IsDir() {
				data.WebSites = append(data.WebSites, WebSite{
					Name:      dirEntry.Name(),
					AccessURL: fmt.Sprintf("/custom-site/%s/", dirEntry.Name()),
				})
			}
		}
	} else if !os.IsNotExist(err) {
		log.Printf("WARNING: Could not read static websites directory '%s': %v", staticFilesDir, err)
	}
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("ERROR: Failed to render library template: %v", err)
		http.Error(w, "Failed to render library page.", http.StatusInternalServerError)
	}
}
