package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxTextFileBytes int64 = 1024 * 1024

type InstanceTextFile struct {
	Name         string `json:"name"`
	RelativePath string `json:"relativePath"`
	Size         int64  `json:"size"`
	ModifiedUnix int64  `json:"modifiedUnix"`
}

type LogsOverview struct {
	Available      bool               `json:"available"`
	DefaultLiveLog string             `json:"defaultLiveLog"`
	LatestCrash    *InstanceTextFile  `json:"latestCrash,omitempty"`
	CrashReports   []InstanceTextFile `json:"crashReports"`
	LogFiles       []InstanceTextFile `json:"logFiles"`
}

type TextFileContent struct {
	Name         string `json:"name"`
	RelativePath string `json:"relativePath"`
	Content      string `json:"content"`
	TotalSize    int64  `json:"totalSize"`
	ModifiedUnix int64  `json:"modifiedUnix"`
	Truncated    bool   `json:"truncated"`
	Missing      bool   `json:"missing"`
}

type LiveLogChunk struct {
	RelativePath string `json:"relativePath"`
	Content      string `json:"content"`
	TotalSize    int64  `json:"totalSize"`
	ModifiedUnix int64  `json:"modifiedUnix"`
}

func (a *App) GetLogsOverview() (*LogsOverview, error) {
	overview := &LogsOverview{
		Available:      a.instanceRoot() != "",
		DefaultLiveLog: filepath.ToSlash(filepath.Join("logs", "latest.log")),
		CrashReports:   []InstanceTextFile{},
		LogFiles:       []InstanceTextFile{},
	}

	if !overview.Available {
		return overview, nil
	}

	logFiles, err := listInstanceTextFiles(filepath.Join(a.instanceRoot(), "logs"), "logs", isRuntimeLogFile)
	if err != nil {
		return nil, err
	}
	crashReports, err := listInstanceTextFiles(filepath.Join(a.instanceRoot(), "crash-reports"), "crash-reports", isCrashReportFile)
	if err != nil {
		return nil, err
	}

	overview.LogFiles = logFiles
	overview.CrashReports = crashReports
	if len(crashReports) > 0 {
		latest := crashReports[0]
		overview.LatestCrash = &latest
	}

	return overview, nil
}

func (a *App) ReadInstanceTextFile(relativePath string) (*TextFileContent, error) {
	absPath, normalized, err := a.resolveInstanceTextPath(relativePath)
	if err != nil {
		return nil, err
	}
	return readTextFileSnapshot(absPath, normalized, maxTextFileBytes)
}

func (a *App) ClearLatestLogContent(relativePath string) (*TextFileContent, error) {
	absPath, normalized, err := a.resolveInstanceTextPath(relativePath)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(normalized) != "logs/latest.log" {
		return nil, fmt.Errorf("only logs/latest.log can be cleared")
	}
	if err := os.WriteFile(absPath, nil, 0644); err != nil {
		return nil, err
	}
	return readTextFileSnapshot(absPath, normalized, maxTextFileBytes)
}

func (a *App) StartLiveLog(relativePath string) (*TextFileContent, error) {
	absPath, normalized, err := a.resolveInstanceTextPath(relativePath)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(strings.ToLower(normalized), "logs/") {
		return nil, fmt.Errorf("live log streaming is only supported for log files")
	}

	snapshot, err := readTextFileSnapshot(absPath, normalized, maxTextFileBytes)
	if err != nil {
		return nil, err
	}

	a.liveLogMu.Lock()
	a.stopLiveLogLocked(false)
	ctx, cancel := context.WithCancel(context.Background())
	a.liveLogCancel = cancel
	a.liveLogMu.Unlock()

	go a.streamLiveLog(ctx, absPath, normalized, snapshot)

	return snapshot, nil
}

func (a *App) StopLiveLog() {
	a.liveLogMu.Lock()
	defer a.liveLogMu.Unlock()
	a.stopLiveLogLocked(true)
}

func (a *App) stopLiveLogLocked(emitEvent bool) {
	if a.liveLogCancel != nil {
		a.liveLogCancel()
		a.liveLogCancel = nil
	}
	if emitEvent && a.ctx != nil {
		runtime.EventsEmit(a.ctx, "logs:stopped")
	}
}

func (a *App) streamLiveLog(ctx context.Context, absPath, relativePath string, snapshot *TextFileContent) {
	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()

	lastSize := snapshot.TotalSize
	lastMissing := snapshot.Missing

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, err := os.Stat(absPath)
			if err != nil {
				if os.IsNotExist(err) {
					if !lastMissing && a.ctx != nil {
						lastMissing = true
						lastSize = 0
						runtime.EventsEmit(a.ctx, "logs:reset", &TextFileContent{
							Name:         filepath.Base(relativePath),
							RelativePath: relativePath,
							Content:      "",
							Missing:      true,
						})
					}
					continue
				}
				continue
			}

			if lastMissing || info.Size() < lastSize {
				resetSnapshot, readErr := readTextFileSnapshot(absPath, relativePath, maxTextFileBytes)
				if readErr != nil {
					continue
				}
				lastMissing = resetSnapshot.Missing
				lastSize = resetSnapshot.TotalSize
				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "logs:reset", resetSnapshot)
				}
				continue
			}

			if info.Size() == lastSize {
				continue
			}

			chunk, totalSize, modifiedUnix, readErr := readTextFileChunk(absPath, lastSize)
			if readErr != nil {
				continue
			}
			lastMissing = false
			lastSize = totalSize
			if chunk == "" || a.ctx == nil {
				continue
			}
			runtime.EventsEmit(a.ctx, "logs:append", LiveLogChunk{
				RelativePath: relativePath,
				Content:      chunk,
				TotalSize:    totalSize,
				ModifiedUnix: modifiedUnix,
			})
		}
	}
}

func (a *App) instanceRoot() string {
	if a.modsDir == "" {
		return ""
	}
	return filepath.Dir(a.modsDir)
}

func (a *App) resolveInstanceTextPath(relativePath string) (string, string, error) {
	root := a.instanceRoot()
	if root == "" {
		return "", "", fmt.Errorf("no instance selected")
	}

	cleanInput := strings.TrimSpace(relativePath)
	if cleanInput == "" {
		return "", "", fmt.Errorf("path is required")
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}

	absPath := cleanInput
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Join(rootAbs, cleanInput)
	}
	absPath, err = filepath.Abs(absPath)
	if err != nil {
		return "", "", err
	}

	relPath, err := filepath.Rel(rootAbs, absPath)
	if err != nil {
		return "", "", err
	}
	relPath = filepath.ToSlash(filepath.Clean(relPath))
	if relPath == "." || strings.HasPrefix(relPath, "../") || relPath == ".." {
		return "", "", fmt.Errorf("path is outside instance directory")
	}
	if !isAllowedInstanceTextPath(relPath) {
		return "", "", fmt.Errorf("unsupported file path")
	}

	return absPath, relPath, nil
}

func listInstanceTextFiles(dirPath, rootPrefix string, include func(string) bool) ([]InstanceTextFile, error) {
	entries := make([]InstanceTextFile, 0)
	if _, err := os.Stat(dirPath); err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, err
	}

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relFromDir, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}
		relPath := filepath.ToSlash(filepath.Join(rootPrefix, relFromDir))
		if !include(relPath) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		entries = append(entries, InstanceTextFile{
			Name:         filepath.Base(path),
			RelativePath: relPath,
			Size:         info.Size(),
			ModifiedUnix: info.ModTime().Unix(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ModifiedUnix == entries[j].ModifiedUnix {
			return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
		}
		return entries[i].ModifiedUnix > entries[j].ModifiedUnix
	})

	return entries, nil
}

func isAllowedInstanceTextPath(relativePath string) bool {
	normalized := strings.ToLower(filepath.ToSlash(filepath.Clean(relativePath)))
	if strings.HasPrefix(normalized, "logs/") {
		return isRuntimeLogFile(normalized)
	}
	if strings.HasPrefix(normalized, "crash-reports/") {
		return isCrashReportFile(normalized)
	}
	return false
}

func isRuntimeLogFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".log" || ext == ".txt"
}

func isCrashReportFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".txt" || ext == ".log"
}

func readTextFileSnapshot(absPath, relativePath string, maxBytes int64) (*TextFileContent, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &TextFileContent{
				Name:         filepath.Base(relativePath),
				RelativePath: relativePath,
				Content:      "",
				Missing:      true,
			}, nil
		}
		return nil, err
	}

	file, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	readSize := info.Size()
	start := int64(0)
	truncated := false
	if maxBytes > 0 && readSize > maxBytes {
		start = readSize - maxBytes
		readSize = maxBytes
		truncated = true
	}

	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	content := string(data)
	if truncated {
		content = fmt.Sprintf("... showing the last %d KB of %d KB ...\n\n%s", maxBytes/1024, info.Size()/1024, content)
	}

	return &TextFileContent{
		Name:         filepath.Base(relativePath),
		RelativePath: relativePath,
		Content:      content,
		TotalSize:    info.Size(),
		ModifiedUnix: info.ModTime().Unix(),
		Truncated:    truncated,
	}, nil
}

func readTextFileChunk(absPath string, offset int64) (string, int64, int64, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return "", 0, 0, err
	}
	if offset > info.Size() {
		offset = info.Size()
	}

	file, err := os.Open(absPath)
	if err != nil {
		return "", 0, 0, err
	}
	defer file.Close()

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return "", 0, 0, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return "", 0, 0, err
	}

	return string(data), info.Size(), info.ModTime().Unix(), nil
}
