package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const maxUpdateSize = 64 << 20

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type githubRelease struct {
	Tag    string         `json:"tag_name"`
	Assets []releaseAsset `json:"assets"`
}

type updater struct {
	client     *http.Client
	releaseURL string
	platform   string
}

func updateApp() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate installed binary: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("resolve installed binary: %w", err)
	}
	u := updater{
		client:     &http.Client{Timeout: 2 * time.Minute},
		releaseURL: "https://api.github.com/repos/Zihad550/go-timer/releases/latest",
		platform:   runtime.GOOS + "_" + runtime.GOARCH,
	}
	return u.update(executable, version, os.Stdout)
}

func (u updater) download(url string, dst io.Writer, limit int64) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "go-timer-updater")
	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	n, err := io.Copy(dst, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		return fmt.Errorf("download exceeds %d bytes", limit)
	}
	return nil
}

func (u updater) update(executable, currentVersion string, output io.Writer) error {
	var metadata bytes.Buffer
	if err := u.download(u.releaseURL, &metadata, 1<<20); err != nil {
		return fmt.Errorf("check latest release: %w", err)
	}
	var release githubRelease
	if err := json.Unmarshal(metadata.Bytes(), &release); err != nil {
		return fmt.Errorf("read latest release: %w", err)
	}
	if release.Tag == "" {
		return fmt.Errorf("latest release has no version tag")
	}
	if strings.TrimPrefix(currentVersion, "v") == strings.TrimPrefix(release.Tag, "v") {
		fmt.Fprintf(output, "Already up to date (%s).\n", release.Tag)
		return nil
	}

	archiveName := "go-timer_" + u.platform + ".tar.gz"
	var archiveURL, checksumsURL string
	for _, asset := range release.Assets {
		switch asset.Name {
		case archiveName:
			archiveURL = asset.URL
		case "checksums.txt":
			checksumsURL = asset.URL
		}
	}
	if archiveURL == "" {
		return fmt.Errorf("release %s has no binary for %s", release.Tag, u.platform)
	}
	if checksumsURL == "" {
		return fmt.Errorf("release %s has no checksums.txt", release.Tag)
	}
	var checksums bytes.Buffer
	if err := u.download(checksumsURL, &checksums, 1<<20); err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	var expectedHash []byte
	for _, line := range strings.Split(checksums.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == archiveName {
			expectedHash, _ = hex.DecodeString(fields[0])
			break
		}
	}
	if len(expectedHash) != sha256.Size {
		return fmt.Errorf("missing or invalid checksum for %s", archiveName)
	}

	archive, err := os.CreateTemp("", "go-timer-update-*.tar.gz")
	if err != nil {
		return fmt.Errorf("create archive file: %w", err)
	}
	defer os.Remove(archive.Name())
	defer archive.Close()
	fmt.Fprintf(output, "Updating %s to %s...\n", currentVersion, release.Tag)
	hash := sha256.New()
	if err := u.download(archiveURL, io.MultiWriter(archive, hash), maxUpdateSize); err != nil {
		return fmt.Errorf("download release: %w", err)
	}
	if !bytes.Equal(hash.Sum(nil), expectedHash) {
		return fmt.Errorf("checksum mismatch for %s", archiveName)
	}
	if _, err := archive.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := installUpdate(archive, executable); err != nil {
		return err
	}
	fmt.Fprintf(output, "Updated to %s. Run go-timer again to use it.\n", release.Tag)
	return nil
}

func installUpdate(archive io.Reader, executable string) error {
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("inspect installed binary: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("installed binary is not a regular file: %s", executable)
	}
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return fmt.Errorf("open release archive: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("release archive does not contain go-timer")
		}
		if err != nil {
			return fmt.Errorf("read release archive: %w", err)
		}
		if header.Name != "go-timer" && header.Name != "./go-timer" {
			continue
		}
		if !header.FileInfo().Mode().IsRegular() || header.Size <= 0 || header.Size > maxUpdateSize {
			return fmt.Errorf("invalid go-timer binary in release archive")
		}
		// Stage beside the executable so rename replaces it atomically.
		replacement, err := os.CreateTemp(filepath.Dir(executable), ".go-timer-update-*")
		if err != nil {
			return fmt.Errorf("create replacement beside %s (installation directory must be writable): %w", executable, err)
		}
		defer os.Remove(replacement.Name())
		defer replacement.Close()
		if _, err := io.CopyN(replacement, tr, header.Size); err != nil {
			return fmt.Errorf("extract update: %w", err)
		}
		if err := replacement.Chmod(info.Mode().Perm()); err != nil {
			return fmt.Errorf("set update permissions: %w", err)
		}
		if err := replacement.Sync(); err != nil {
			return fmt.Errorf("sync update: %w", err)
		}
		if err := replacement.Close(); err != nil {
			return fmt.Errorf("close update: %w", err)
		}
		if err := os.Rename(replacement.Name(), executable); err != nil {
			return fmt.Errorf("replace %s: %w", executable, err)
		}
		return nil
	}
}
