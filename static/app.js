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
        reject("Upload failed: " + xhr.responseText);
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
