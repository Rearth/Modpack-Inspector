package embeddings

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	ortVersion = "1.24.1"
	modelName  = "all-MiniLM-L6-v2"

	modelURL = "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx"
	vocabURL = "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/vocab.txt"
)

// ProgressFunc is called during downloads with a stage description and 0-1 progress.
type ProgressFunc func(stage string, pct float64)

// ModelDir returns the directory where model files are stored.
func ModelDir(dataDir string) string {
	return filepath.Join(dataDir, "models", modelName)
}

// RuntimePath returns the platform-specific path for the ONNX Runtime shared library.
func RuntimePath(dataDir string) string {
	return filepath.Join(dataDir, "models", runtimeFileName())
}

func runtimeFileName() string {
	switch runtime.GOOS {
	case "windows":
		return "onnxruntime.dll"
	case "linux":
		return "libonnxruntime.so"
	case "darwin":
		return "libonnxruntime.dylib"
	}
	return "onnxruntime"
}

func ortDownloadURL() string {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	switch goos {
	case "windows":
		arch := "x64"
		if goarch == "arm64" {
			arch = "arm64"
		}
		return fmt.Sprintf(
			"https://github.com/microsoft/onnxruntime/releases/download/v%s/onnxruntime-win-%s-%s.zip",
			ortVersion, arch, ortVersion,
		)
	case "linux":
		arch := "x64"
		if goarch == "arm64" {
			arch = "aarch64"
		}
		return fmt.Sprintf(
			"https://github.com/microsoft/onnxruntime/releases/download/v%s/onnxruntime-linux-%s-%s.tgz",
			ortVersion, arch, ortVersion,
		)
	case "darwin":
		// macOS universal binary
		return fmt.Sprintf(
			"https://github.com/microsoft/onnxruntime/releases/download/v%s/onnxruntime-osx-universal2-%s.tgz",
			ortVersion, ortVersion,
		)
	}
	return ""
}

// EnsureModelFiles downloads the ONNX model, vocab, and runtime if not already present.
func EnsureModelFiles(dataDir string, progress ProgressFunc) error {
	modelDir := ModelDir(dataDir)
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return fmt.Errorf("creating model dir: %w", err)
	}

	// Vocab
	vocabPath := filepath.Join(modelDir, "vocab.txt")
	if _, err := os.Stat(vocabPath); os.IsNotExist(err) {
		progress("Downloading vocabulary...", 0)
		if err := downloadFile(vocabURL, vocabPath); err != nil {
			return fmt.Errorf("downloading vocab: %w", err)
		}
		progress("Downloading vocabulary...", 1)
	}

	// Model
	modelPath := filepath.Join(modelDir, "model.onnx")
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		progress("Downloading embedding model (~90 MB)...", 0)
		if err := downloadFile(modelURL, modelPath); err != nil {
			os.Remove(modelPath) // cleanup partial
			return fmt.Errorf("downloading model: %w", err)
		}
		progress("Downloading embedding model...", 1)
	}

	// ONNX Runtime
	rtPath := RuntimePath(dataDir)
	if _, err := os.Stat(rtPath); os.IsNotExist(err) {
		progress("Downloading ONNX Runtime (~60 MB)...", 0)
		if err := downloadRuntime(dataDir); err != nil {
			return fmt.Errorf("downloading runtime: %w", err)
		}
		progress("Downloading ONNX Runtime...", 1)
	}

	return nil
}

func downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func downloadRuntime(dataDir string) error {
	url := ortDownloadURL()
	if url == "" {
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	if strings.HasSuffix(url, ".zip") {
		return downloadAndExtractZip(url, dataDir)
	}
	// tgz for linux/mac — for now only support Windows zip
	return fmt.Errorf("tgz extraction not yet implemented for %s; please manually place %s in %s",
		runtime.GOOS, runtimeFileName(), filepath.Join(dataDir, "models"))
}

func downloadAndExtractZip(url, dataDir string) error {
	tmpFile, err := os.CreateTemp("", "onnxruntime-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	resp, err := http.Get(url)
	if err != nil {
		tmpFile.Close()
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	return extractDLLFromZip(tmpPath, RuntimePath(dataDir))
}

func extractDLLFromZip(zipPath, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	target := runtimeFileName()
	for _, f := range r.File {
		// Match by filename at end of path (e.g. "onnxruntime-win-x64-1.20.1/lib/onnxruntime.dll")
		if !strings.HasSuffix(f.Name, "/"+target) && filepath.Base(f.Name) != target {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			return err
		}
		_, copyErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		return copyErr
	}

	return fmt.Errorf("%s not found in zip archive", target)
}
