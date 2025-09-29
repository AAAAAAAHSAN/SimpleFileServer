package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	http.HandleFunc("/upload", uploadHandler) // handles chunked uploads
	http.HandleFunc("/status", statusHandler) // tells client uploaded size
	http.HandleFunc("/download/", downloadHandler)

	fmt.Println("🚀 Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

// Home page with upload form + progress
func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	fmt.Fprintln(w, `
	<h1>Multi File Upload with Resume</h1>
	<form id="uploadForm" enctype="multipart/form-data">
		<input type="file" id="fileInput" multiple />
		<input type="submit" value="Upload" />
	</form>
	<div id="progressContainer"></div>

<script>
const form = document.getElementById("uploadForm");
const progressContainer = document.getElementById("progressContainer");

form.addEventListener("submit", async function(e) {
  e.preventDefault();
  const files = document.getElementById("fileInput").files;
  if (!files.length) return alert("Select files first");

  progressContainer.innerHTML = "";

  for (const file of files) {
    // UI for each file
    const wrapper = document.createElement("div");
    wrapper.style.margin = "10px 0";

    const label = document.createElement("span");
    label.textContent = file.name + ": ";

    const progressBar = document.createElement("progress");
    progressBar.max = 100;
    progressBar.value = 0;
    progressBar.style.width = "300px";

    const status = document.createElement("span");
    status.style.marginLeft = "10px";

    wrapper.appendChild(label);
    wrapper.appendChild(progressBar);
    wrapper.appendChild(status);
    progressContainer.appendChild(wrapper);

    await uploadFile(file, progressBar, status);
  }
});

async function uploadFile(file, progressBar, status) {
  const chunkSize = 1024 * 512; // 512KB chunks
  let uploadedBytes = await getUploadedBytes(file.name);

  for (let start = uploadedBytes; start < file.size; start += chunkSize) {
    const chunk = file.slice(start, start + chunkSize);
    await uploadChunk(file.name, start, chunk, file.size, progressBar, status);
  }

  progressBar.value = 100;
  status.textContent = "Done!";
}

function uploadChunk(filename, start, chunk, totalSize, progressBar, status) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const formData = new FormData();
    formData.append("file", chunk);
    formData.append("filename", filename);
    formData.append("start", start);

    xhr.open("POST", "/upload", true);

    xhr.upload.onprogress = function(e) {
      if (e.lengthComputable) {
        const overall = ((start + e.loaded) / totalSize) * 100;
        progressBar.value = overall;
        status.textContent = overall.toFixed(2) + "%";
      }
    };

    xhr.onload = function() {
      if (xhr.status === 200) {
        resolve();
      } else {
        reject("Upload failed: " + xhr.statusText);
      }
    };

    xhr.onerror = function() {
      reject("Network error");
    };

    xhr.send(formData);
  });
}

async function getUploadedBytes(filename) {
  try {
    const res = await fetch("/status?filename=" + encodeURIComponent(filename));
    if (!res.ok) return 0;
    const text = await res.text();
    return parseInt(text) || 0;
  } catch {
    return 0;
  }
}
</script>
	`)
}

// List uploaded files
func filesHandler(w http.ResponseWriter, r *http.Request) {
	files, _ := os.ReadDir(uploadDir)

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintln(w, "<h2>Uploaded Files</h2><ul>")
	for _, f := range files {
		if !f.IsDir() {
			fmt.Fprintf(w, `<li><a href="/download/%s">%s</a></li>`, f.Name(), f.Name())
		}
	}
	fmt.Fprintln(w, "</ul>")
}

// Upload chunks
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(64 << 20) // 64MB memory limit

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := r.FormValue("filename")
	startStr := r.FormValue("start")
	start, _ := strconv.ParseInt(startStr, 10, 64)

	dstPath := filepath.Join(uploadDir, filename)
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(w, "Could not open file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err = dst.Seek(start, 0); err != nil {
		http.Error(w, "Seek failed", http.StatusInternalServerError)
		return
	}

	if _, err = io.Copy(dst, file); err != nil {
		http.Error(w, "Write failed", http.StatusInternalServerError)
		return
	}
}

// Status endpoint for resume support
func statusHandler(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Query().Get("filename")
	if filename == "" {
		http.Error(w, "Missing filename", http.StatusBadRequest)
		return
	}

	path := filepath.Join(uploadDir, filename)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		fmt.Fprint(w, "0")
		return
	}
	fmt.Fprint(w, info.Size())
}

// Download file
func downloadHandler(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	path := filepath.Join(uploadDir, filename)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	http.ServeFile(w, r, path)
}
