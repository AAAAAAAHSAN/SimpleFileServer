package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

const uploadDir = "./uploads"

func main() {
	// Ensure uploads dir exists
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		if err := os.Mkdir(uploadDir, 0755); err != nil {
			log.Fatalf("Failed to create upload dir: %v", err)
		}
	}

	// API endpoints
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/list", handleList)
	http.HandleFunc("/download", handleDownload)

	// Static files (HTML, JS, CSS)
	http.Handle("/", http.FileServer(http.Dir("static")))

	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))

	fmt.Println("🚀 Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// Upload handler for resumable uploads
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	// Parse form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := r.FormValue("filename")
	startStr := r.FormValue("start")
	if filename == "" || startStr == "" {
		http.Error(w, "Missing filename/start", http.StatusBadRequest)
		return
	}

	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid start offset", http.StatusBadRequest)
		return
	}

	dstPath := filepath.Join(uploadDir, filepath.Base(filename))
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "Failed to open destination", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := dst.Seek(start, io.SeekStart); err != nil {
		http.Error(w, "Failed to seek", http.StatusInternalServerError)
		return
	}

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Failed to write chunk", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "Missing file parameter", http.StatusBadRequest)
		return
	}

	path := filepath.Join(uploadDir, filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	http.ServeFile(w, r, path)
}

// Status handler (returns current uploaded size)
func statusHandler(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		http.Error(w, "Missing filename", http.StatusBadRequest)
		return
	}

	dstPath := filepath.Join(uploadDir, filepath.Base(filename))
	info, err := os.Stat(dstPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprint(w, "0")
			return
		}
		http.Error(w, "Could not stat file", http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, info.Size())
}

type FileInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

func handleList(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		http.Error(w, "Could not read upload directory", http.StatusInternalServerError)
		return
	}

	var files []FileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// get size
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, FileInfo{
			Name: e.Name(),
			Size: info.Size(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}
