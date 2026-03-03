package transfer_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/ValeryCherneykin/ned/internal/backend/mock"
	"github.com/ValeryCherneykin/ned/internal/transfer"
)

// ─────────────────────────────────────────────
// Download
// ─────────────────────────────────────────────

func TestDownload_ExistingFile(t *testing.T) {
	t.Parallel()

	content := []byte("KEY=value\nSECRET=abc123\n")
	b := mock.New(map[string][]byte{"/etc/.env": content})

	tmpPath, isNew, err := transfer.Download(b, "/etc/.env")
	if err != nil {
		t.Fatalf("Download() error: %v", err)
	}

	defer func() { _ = os.Remove(tmpPath) }()

	if isNew {
		t.Error("isNew = true, want false for existing file")
	}

	got, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("ReadFile(tmp): %v", err)
	}

	if !bytes.Equal(got, content) {
		t.Errorf("temp content mismatch\ngot:  %q\nwant: %q", got, content)
	}
}

func TestDownload_NewFile(t *testing.T) {
	t.Parallel()

	b := mock.New(nil)

	tmpPath, isNew, err := transfer.Download(b, "/tmp/new-file.txt")
	if err != nil {
		t.Fatalf("Download() error: %v", err)
	}

	defer func() { _ = os.Remove(tmpPath) }()

	if !isNew {
		t.Error("isNew = false, want true for missing file")
	}

	info, err := os.Stat(tmpPath)
	if err != nil {
		t.Fatalf("Stat(tmp): %v", err)
	}

	if info.Size() != 0 {
		t.Errorf("new file size = %d, want 0", info.Size())
	}
}

func TestDownload_TempFileNameContainsBasename(t *testing.T) {
	t.Parallel()

	b := mock.New(map[string][]byte{"/etc/nginx/nginx.conf": []byte("worker_processes 1;")})

	tmpPath, _, err := transfer.Download(b, "/etc/nginx/nginx.conf")
	if err != nil {
		t.Fatalf("Download() error: %v", err)
	}

	defer func() { _ = os.Remove(tmpPath) }()

	if !strings.Contains(tmpPath, "nginx.conf") {
		t.Errorf("temp path %q does not contain file basename", tmpPath)
	}
}

func TestDownload_ReadError(t *testing.T) {
	t.Parallel()

	b := mock.New(nil)
	b.ReadErr = os.ErrPermission // not os.ErrNotExist — should propagate

	_, _, err := transfer.Download(b, "/etc/secret")
	if err == nil {
		t.Error("Download() expected error on ReadErr, got nil")
	}
}

// ─────────────────────────────────────────────
// Upload
// ─────────────────────────────────────────────

func TestUpload_WritesContentToBackend(t *testing.T) {
	t.Parallel()

	content := []byte("APP_ENV=production\nDB_URL=postgres://...\n")

	tmp, err := os.CreateTemp("", "ned-upload-test-")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err = tmp.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if err = tmp.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b := mock.New(nil)

	if err = transfer.Upload(b, tmp.Name(), "/app/.env"); err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	got := b.Get("/app/.env")
	if !bytes.Equal(got, content) {
		t.Errorf("remote content mismatch\ngot:  %q\nwant: %q", got, content)
	}
}

func TestUpload_MissingLocalFile(t *testing.T) {
	t.Parallel()

	b := mock.New(nil)

	err := transfer.Upload(b, "/nonexistent/path/file.txt", "/remote/file.txt")
	if err == nil {
		t.Error("Upload() expected error for missing local file, got nil")
	}
}

func TestUpload_WriteError(t *testing.T) {
	t.Parallel()

	tmp, err := os.CreateTemp("", "ned-upload-err-")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}

	defer func() { _ = os.Remove(tmp.Name()) }()

	if err = tmp.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	b := mock.New(nil)
	b.WriteErr = os.ErrPermission

	if err = transfer.Upload(b, tmp.Name(), "/remote/file.txt"); err == nil {
		t.Error("Upload() expected error on WriteErr, got nil")
	}
}

// ─────────────────────────────────────────────
// Benchmarks
// ─────────────────────────────────────────────

func BenchmarkDownload_1KB(b *testing.B)  { benchmarkDownload(b, 1*1024) }
func BenchmarkDownload_1MB(b *testing.B)  { benchmarkDownload(b, 1*1024*1024) }
func BenchmarkDownload_10MB(b *testing.B) { benchmarkDownload(b, 10*1024*1024) }

func benchmarkDownload(b *testing.B, size int) {
	b.Helper()
	b.ReportAllocs()

	content := bytes.Repeat([]byte("x"), size)
	bk := mock.New(map[string][]byte{"/bench/file": content})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tmpPath, _, err := transfer.Download(bk, "/bench/file")
		if err != nil {
			b.Fatalf("Download error: %v", err)
		}

		_ = os.Remove(tmpPath)
	}

	b.SetBytes(int64(size))
}

func BenchmarkUpload_1KB(b *testing.B)  { benchmarkUpload(b, 1*1024) }
func BenchmarkUpload_1MB(b *testing.B)  { benchmarkUpload(b, 1*1024*1024) }
func BenchmarkUpload_10MB(b *testing.B) { benchmarkUpload(b, 10*1024*1024) }

func benchmarkUpload(b *testing.B, size int) {
	b.Helper()
	b.ReportAllocs()

	content := bytes.Repeat([]byte("y"), size)

	tmp, err := os.CreateTemp("", "ned-bench-upload-")
	if err != nil {
		b.Fatalf("CreateTemp: %v", err)
	}

	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err = tmp.Write(content); err != nil {
		b.Fatalf("Write: %v", err)
	}

	if err = tmp.Close(); err != nil {
		b.Fatalf("Close: %v", err)
	}

	bk := mock.New(nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err = transfer.Upload(bk, tmp.Name(), "/bench/file"); err != nil {
			b.Fatalf("Upload error: %v", err)
		}
	}

	b.SetBytes(int64(size))
}
