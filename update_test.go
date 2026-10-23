package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type updateTransport func(*http.Request) (*http.Response, error)

func (f updateTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func updateArchive(t *testing.T, name string, kind byte) []byte {
	t.Helper()
	var data bytes.Buffer
	gz := gzip.NewWriter(&data)
	tw := tar.NewWriter(gz)
	payload := []byte("new binary")
	header := &tar.Header{Name: name, Mode: 0755, Size: int64(len(payload)), Typeflag: kind}
	if kind == tar.TypeSymlink {
		header.Linkname = "/tmp/other-binary"
		header.Size = 0
	}
	if err := tw.WriteHeader(header); err != nil {
		t.Fatal(err)
	}
	if header.Size > 0 {
		if _, err := tw.Write(payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestUpdate(t *testing.T) {
	archiveName := "go-timer_linux_amd64.tar.gz"
	validArchive := updateArchive(t, "go-timer", tar.TypeReg)
	tests := []struct {
		name           string
		currentVersion string
		platform       string
		archive        []byte
		omitAsset      string
		badChecksum    bool
		missingHash    bool
		failPath       string
		wantError      string
		wantUnchanged  bool
	}{
		{name: "install release", archive: validArchive},
		{name: "dev build", currentVersion: "dev", archive: validArchive},
		{name: "dot prefixed entry", archive: updateArchive(t, "./go-timer", tar.TypeReg)},
		{name: "already current", currentVersion: "1.2.3", wantUnchanged: true},
		{name: "already current with v", currentVersion: "v1.2.3", wantUnchanged: true},
		{name: "unsupported platform", platform: "linux_arm64", wantError: "no binary for linux_arm64"},
		{name: "missing archive", omitAsset: archiveName, wantError: "no binary"},
		{name: "missing checksums", omitAsset: "checksums.txt", wantError: "no checksums.txt"},
		{name: "missing checksum entry", missingHash: true, wantError: "missing or invalid checksum"},
		{name: "checksum mismatch", archive: validArchive, badChecksum: true, wantError: "checksum mismatch"},
		{name: "invalid gzip", archive: []byte("broken"), wantError: "open release archive"},
		{name: "missing binary", archive: updateArchive(t, "README.md", tar.TypeReg), wantError: "does not contain go-timer"},
		{name: "path traversal", archive: updateArchive(t, "../go-timer", tar.TypeReg), wantError: "does not contain go-timer"},
		{name: "symlink entry", archive: updateArchive(t, "go-timer", tar.TypeSymlink), wantError: "invalid go-timer binary"},
		{name: "API failure", failPath: "/latest", wantError: "check latest release"},
		{name: "checksums failure", failPath: "/checksums", wantError: "download checksums"},
		{name: "archive failure", failPath: "/archive", wantError: "download release"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			executable := filepath.Join(dir, "go-timer")
			if err := os.WriteFile(executable, []byte("old binary"), 0751); err != nil {
				t.Fatal(err)
			}
			release := githubRelease{Tag: "v1.2.3"}
			for name, url := range map[string]string{archiveName: "https://example.test/archive", "checksums.txt": "https://example.test/checksums"} {
				if name != tt.omitAsset {
					release.Assets = append(release.Assets, releaseAsset{Name: name, URL: url})
				}
			}
			metadata, err := json.Marshal(release)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256(tt.archive)
			checksums := fmt.Sprintf("%x  %s\n", hash, archiveName)
			if tt.badChecksum {
				checksums = fmt.Sprintf("%064d  %s\n", 0, archiveName)
			}
			if tt.missingHash {
				checksums = fmt.Sprintf("%x  other.tar.gz\n", hash)
			}
			requests := 0
			client := &http.Client{Transport: updateTransport(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Header.Get("User-Agent") != "go-timer-updater" {
					t.Error("missing updater user agent")
				}
				status := http.StatusOK
				if req.URL.Path == tt.failPath {
					status = http.StatusForbidden
				}
				bodies := map[string][]byte{"/latest": metadata, "/checksums": []byte(checksums), "/archive": tt.archive}
				body, ok := bodies[req.URL.Path]
				if !ok {
					t.Fatalf("unexpected request: %s", req.URL)
				}
				return &http.Response{StatusCode: status, Status: http.StatusText(status), Body: io.NopCloser(bytes.NewReader(body)), Header: make(http.Header)}, nil
			})}
			platform := tt.platform
			if platform == "" {
				platform = "linux_amd64"
			}
			current := tt.currentVersion
			if current == "" {
				current = "1.0.0"
			}
			u := updater{client: client, releaseURL: "https://example.test/latest", platform: platform}
			var output bytes.Buffer
			err = u.update(executable, current, &output)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			wantBinary := "new binary"
			if tt.wantUnchanged || tt.wantError != "" {
				wantBinary = "old binary"
			}
			actual, err := os.ReadFile(executable)
			if err != nil || string(actual) != wantBinary {
				t.Fatalf("binary = %q, error = %v; want %q", actual, err, wantBinary)
			}
			info, err := os.Stat(executable)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0751 {
				t.Fatalf("permissions changed: %o", info.Mode().Perm())
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary replacement left behind: %v, %v", entries, err)
			}
			if tt.wantUnchanged && (requests != 1 || !strings.Contains(output.String(), "Already up to date")) {
				t.Fatalf("current version downloaded assets: requests=%d, output=%q", requests, output.String())
			}
		})
	}
}

func TestUpdateDownloadLimit(t *testing.T) {
	u := updater{client: &http.Client{Transport: updateTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("too large"))}, nil
	})}}
	if err := u.download("https://example.test", io.Discard, 4); err == nil {
		t.Fatal("expected oversized download to fail")
	}
}
