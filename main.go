package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const uploadDir = "./uploads"

func main() {
	// Ensure uploads directory exists
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		err = os.Mkdir(uploadDir, 0755)
		if err != nil {
			log.Fatalf("Could not create upload dir: %v", err)
		}
	}

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/files", filesHandler)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/download/", downloadHandler) // NEW

	fmt.Println("🚀 Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

// Home route
func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	fmt.Fprintln(w, "<h1>Welcome to Ahsan's File Sharing Server! </h1>")
	fmt.Fprintln(w, "<h3>Endpoints:</h3>")
	fmt.Fprintln(w, "<ul>")
	fmt.Fprintln(w, `<li><a href="/files">Files list</a></li><br/>`)
	fmt.Fprintln(w, `<li><a href="/upload">Upload a file (form)</a></li>`)
	//fmt.Fprintln(w, `<li>POST /upload → handle file upload (via form)</li>`)
	//fmt.Fprintln(w, `<li><a href="/download/">GET /download/{file}</a> → download a file</li>`)
	fmt.Fprintln(w, "</ul>")
}

// List uploaded files (HTML with links)
func filesHandler(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(uploadDir)
	if err != nil {
		http.Error(w, "Could not read upload directory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintln(w, "<h2>Uploaded Files</h2>")

	if len(files) == 0 {
		fmt.Fprintln(w, "<p>No files uploaded yet.</p>")
		return
	}

	fmt.Fprintln(w, "<ul>")
	for _, f := range files {
		name := f.Name()
		if name == ".DS_Store" {
			continue //skip hidden files
		}

		if !f.IsDir() {
			link := fmt.Sprintf("/download/%s", f.Name())
			fmt.Fprintf(w, `<li><a href="%s">%s</a></li>`, link, f.Name())
		}
	}
	fmt.Fprintln(w, "</ul>")
}

// Upload form & handler
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// Show upload form
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `
			<h2>Upload File</h2>
<form id="uploadForm" enctype="multipart/form-data">
    <input type="file" name="file" id="fileInput" />
    <input type="submit" value="Upload" />
</form>

<progress id="progressBar" value="0" max="100" style="width:100%; display:block; margin-top:10px;"></progress>
<p id="status"></p>

<script>
const form = document.getElementById('uploadForm');
const progressBar = document.getElementById('progressBar');
const status = document.getElementById('status');

form.addEventListener('submit', function(e) {
    e.preventDefault();

    const file = document.getElementById('fileInput').files[0];
    if (!file) return alert("Select a file first");

    const xhr = new XMLHttpRequest();
    xhr.open('POST', '/upload', true);

    // Progress event
    xhr.upload.onprogress = function(e) {
        if (e.lengthComputable) {
            const percent = (e.loaded / e.total) * 100;
            progressBar.value = percent;
            status.innerText = Math.round(percent) + "% uploaded";
        }
    };

    xhr.onload = function() {
        if (xhr.status === 200) {
            status.innerText = "Upload complete!";
            progressBar.value = 100;
        } else {
            status.innerText = "Upload failed: " + xhr.responseText;
        }
    };

    const formData = new FormData();
    formData.append('file', file);
    xhr.send(formData);
});
</script>
		`)
		return
	}

	if r.Method == "POST" {
		// Limit request body to 3GB
		const maxUpload = 3 << 30 // 3 GB
		r.Body = http.MaxBytesReader(w, r.Body, maxUpload)

		// Parse multipart form but keep only 64 MB in memory
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			http.Error(w, "File too big or invalid form", http.StatusBadRequest)
			return
		}

		// Get uploaded file
		file, handler, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Could not get uploaded file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Create destination file
		dstPath := filepath.Join(uploadDir, handler.Filename)
		dst, err := os.Create(dstPath)
		if err != nil {
			http.Error(w, "Could not create file", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		// Stream file in chunks to stay under 64 MB RAM
		buf := make([]byte, 8<<20) // 8 MB buffer
		for {
			n, err := file.Read(buf)
			if n > 0 {
				if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
					http.Error(w, "Failed to write file", http.StatusInternalServerError)
					return
				}
			}
			if err != nil {
				if err == io.EOF {
					break
				} else {
					http.Error(w, "Error reading file", http.StatusInternalServerError)
					return
				}
			}
		}

		fmt.Fprintf(w, "✅ Uploaded successfully: %s\n", handler.Filename)
	}
}

// Download handler
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	if filename == "" {
		http.Error(w, "Filename not provided", http.StatusBadRequest)
		return
	}

	filepath := filepath.Join(uploadDir, filename)
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	http.ServeFile(w, r, filepath)
}
