package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveInstanceTextPathRejectsTraversal(t *testing.T) {
	app := &App{modsDir: filepath.Join(t.TempDir(), "mods")}

	_, _, err := app.resolveInstanceTextPath("../outside.txt")
	if err == nil {
		t.Fatal("expected traversal path to be rejected")
	}

	_, normalized, err := app.resolveInstanceTextPath(filepath.Join("logs", "latest.log"))
	if err != nil {
		t.Fatalf("expected logs/latest.log to resolve, got %v", err)
	}
	if normalized != "logs/latest.log" {
		t.Fatalf("expected normalized log path, got %q", normalized)
	}
}

func TestListInstanceTextFilesSortsNewestFirst(t *testing.T) {
	root := t.TempDir()
	logsDir := filepath.Join(root, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}

	older := filepath.Join(logsDir, "debug.log")
	newer := filepath.Join(logsDir, "latest.log")
	ignored := filepath.Join(logsDir, "latest.log.gz")

	if err := os.WriteFile(older, []byte("older"), 0644); err != nil {
		t.Fatalf("write older: %v", err)
	}
	if err := os.WriteFile(newer, []byte("newer"), 0644); err != nil {
		t.Fatalf("write newer: %v", err)
	}
	if err := os.WriteFile(ignored, []byte("ignored"), 0644); err != nil {
		t.Fatalf("write ignored: %v", err)
	}

	oldTime := time.Now().Add(-2 * time.Hour)
	newTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(older, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes older: %v", err)
	}
	if err := os.Chtimes(newer, newTime, newTime); err != nil {
		t.Fatalf("chtimes newer: %v", err)
	}

	files, err := listInstanceTextFiles(logsDir, "logs", isRuntimeLogFile)
	if err != nil {
		t.Fatalf("list logs: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 text log files, got %d", len(files))
	}
	if files[0].RelativePath != "logs/latest.log" {
		t.Fatalf("expected newest file first, got %q", files[0].RelativePath)
	}
	if files[1].RelativePath != "logs/debug.log" {
		t.Fatalf("expected older file second, got %q", files[1].RelativePath)
	}
}

func TestReadTextFileSnapshotMarksTruncation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "latest.log")
	content := strings.Repeat("0123456789", 300)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	snapshot, err := readTextFileSnapshot(path, "logs/latest.log", 128)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if !snapshot.Truncated {
		t.Fatal("expected snapshot to be truncated")
	}
	if !strings.Contains(snapshot.Content, "showing the last") {
		t.Fatalf("expected truncation banner, got %q", snapshot.Content)
	}
	if snapshot.TotalSize != int64(len(content)) {
		t.Fatalf("expected total size %d, got %d", len(content), snapshot.TotalSize)
	}
}

func TestClearLatestLogContentOnlyAllowsLatestLog(t *testing.T) {
	root := t.TempDir()
	logsDir := filepath.Join(root, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	latestPath := filepath.Join(logsDir, "latest.log")
	if err := os.WriteFile(latestPath, []byte("hello world"), 0644); err != nil {
		t.Fatalf("write latest: %v", err)
	}

	app := &App{modsDir: filepath.Join(root, "mods")}
	snapshot, err := app.ClearLatestLogContent(filepath.Join("logs", "latest.log"))
	if err != nil {
		t.Fatalf("clear latest: %v", err)
	}
	if snapshot.TotalSize != 0 || snapshot.Content != "" {
		t.Fatalf("expected cleared latest log, got size=%d content=%q", snapshot.TotalSize, snapshot.Content)
	}

	debugPath := filepath.Join(logsDir, "debug.log")
	if err := os.WriteFile(debugPath, []byte("keep me"), 0644); err != nil {
		t.Fatalf("write debug: %v", err)
	}
	if _, err := app.ClearLatestLogContent(filepath.Join("logs", "debug.log")); err == nil {
		t.Fatal("expected non-latest log clear to be rejected")
	}
}
