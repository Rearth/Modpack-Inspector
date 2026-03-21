package embeddings

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
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
	if strings.HasSuffix(url, ".tgz") {
		return downloadAndExtractTarGz(url, dataDir)
	}

	return fmt.Errorf("unsupported runtime archive format for %s", url)
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

	return extractRuntimeFromZip(tmpPath, RuntimePath(dataDir))
}

func downloadAndExtractTarGz(url, dataDir string) error {
	tmpFile, err := os.CreateTemp("", "onnxruntime-*.tgz")
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

	return extractRuntimeFromTarGz(tmpPath, RuntimePath(dataDir))
}

func extractRuntimeFromZip(zipPath, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	target := runtimeFileName()
	for _, f := range r.File {
		if !archiveEntryMatches(f.Name, target) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		return writeArchiveEntry(rc, destPath)
	}

	return fmt.Errorf("%s not found in zip archive", target)

}

func extractRuntimeFromTarGz(tgzPath, destPath string) error {
	file, err := os.Open(tgzPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	target := runtimeFileName()

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		if !archiveEntryMatches(header.Name, target) {
			continue
		}

		return writeArchiveEntry(tarReader, destPath)
	}

	return fmt.Errorf("%s not found in tar.gz archive", target)
}

func archiveEntryMatches(name, target string) bool {
	return strings.HasSuffix(name, "/"+target) || filepath.Base(name) == target
}

func writeArchiveEntry(reader io.Reader, destPath string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, reader)
	return err
}
