package manager

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 2 * time.Minute}

// DownloadAndExtractRelease downloads a release archive and extracts it to a temp directory.
// Returns the extraction directory on success.
func DownloadAndExtractRelease(releaseURL, platform, arch string) (string, error) {
	assetURL, checksum, err := findAssetInfo(releaseURL, platform, arch)
	if err != nil {
		return "", err
	}

	tmpPattern := "claw-release-*"
	if u, perr := url.Parse(assetURL); perr == nil {
		base := filepath.Base(u.Path)
		lbase := strings.ToLower(base)
		switch {
		case strings.HasSuffix(lbase, ".zip"):
			tmpPattern += ".zip"
		case strings.HasSuffix(lbase, ".tar.gz") || strings.HasSuffix(lbase, ".tgz"):
			tmpPattern += ".tar.gz"
		case strings.HasSuffix(lbase, ".tar"):
			tmpPattern += ".tar"
		default:
			tmpPattern += ".archive"
		}
	} else {
		tmpPattern += ".archive"
	}

	tmpFile, err := os.CreateTemp("", tmpPattern)
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	defer tmpFile.Close()

	resp, err := httpClient.Get(assetURL)
	if err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to download asset: status %d", resp.StatusCode)
	}

	h := sha256.New()
	pw := &progressWriter{total: resp.ContentLength}
	mw := io.MultiWriter(tmpFile, h, pw)
	if _, err = io.Copy(mw, resp.Body); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	pw.Finish()

	if checksum != "" {
		got := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(got, checksum) {
			os.Remove(tmpPath)
			return "", fmt.Errorf("checksum mismatch: got %s expected %s", got, checksum)
		}
	}

	destDir, err := os.MkdirTemp("", "claw-extract-*")
	if err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	if err := extractArchive(tmpPath, destDir); err != nil {
		os.Remove(tmpPath)
		os.RemoveAll(destDir)
		return "", err
	}

	_ = os.Remove(tmpPath)
	return destDir, nil
}

func findAssetInfo(releaseURL, platform, arch string) (string, string, error) {
	apiURL := buildReleaseAPIURL(releaseURL)
	if apiURL == "" {
		return "", "", fmt.Errorf("invalid release URL")
	}

	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to query releases: status %d", resp.StatusCode)
	}

	var data struct {
		Assets []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Digest             string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", "", err
	}

	platformLower := strings.ToLower(platform)
	archLower := strings.ToLower(arch)

	var platformIdx []int
	for i, a := range data.Assets {
		n := strings.ToLower(a.Name)
		if platform == "" || strings.Contains(n, platformLower) {
			platformIdx = append(platformIdx, i)
		}
	}

	if len(platformIdx) == 0 {
		return "", "", fmt.Errorf("no release asset matching platform %q and arch %q", platform, arch)
	}

	var candidates []int
	if arch != "" {
		for _, i := range platformIdx {
			n := strings.ToLower(data.Assets[i].Name)
			if strings.Contains(n, archLower) || strings.Contains(n, archAlias(archLower)) {
				candidates = append(candidates, i)
			}
		}
	}
	if len(candidates) == 0 {
		candidates = platformIdx
	}

	// Prefer tar.gz, then tar, then zip
	for _, i := range candidates {
		name := strings.ToLower(data.Assets[i].Name)
		if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") {
			return data.Assets[i].BrowserDownloadURL, data.Assets[i].Digest, nil
		}
	}
	for _, i := range candidates {
		name := strings.ToLower(data.Assets[i].Name)
		if strings.HasSuffix(name, ".tar") {
			return data.Assets[i].BrowserDownloadURL, data.Assets[i].Digest, nil
		}
	}
	for _, i := range candidates {
		name := strings.ToLower(data.Assets[i].Name)
		if strings.HasSuffix(name, ".zip") {
			return data.Assets[i].BrowserDownloadURL, data.Assets[i].Digest, nil
		}
	}

	return "", "", fmt.Errorf("no suitable asset found")
}

func archAlias(arch string) string {
	aliases := map[string]string{
		"amd64":  "x86_64",
		"x86_64": "amd64",
		"arm64":  "aarch64",
		"aarch64": "arm64",
	}
	if v, ok := aliases[arch]; ok {
		return v
	}
	return arch
}

func buildReleaseAPIURL(releaseURL string) string {
	if strings.Contains(releaseURL, "api.github.com") {
		return releaseURL
	}
	u, err := url.Parse(releaseURL)
	if err != nil {
		return ""
	}
	if u.Host != "github.com" {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	owner := parts[0]
	repo := parts[1]
	if len(parts) >= 5 && parts[2] == "releases" && parts[3] == "tags" {
		tag := parts[4]
		return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, tag)
	}
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
}

func extractArchive(archivePath, destDir string) error {
	lower := strings.ToLower(archivePath)
	if strings.HasSuffix(lower, ".zip") {
		return extractZip(archivePath, destDir)
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return extractTarGz(archivePath, destDir)
	}
	if strings.HasSuffix(lower, ".tar") {
		return extractTar(archivePath, destDir)
	}
	return extractTarGz(archivePath, destDir)
}

func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()
	destClean := filepath.Clean(destDir)
	for _, f := range r.File {
		target := filepath.Clean(filepath.Join(destClean, f.Name))
		if !strings.HasPrefix(target, destClean+string(os.PathSeparator)) && target != destClean {
			return fmt.Errorf("path traversal detected: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, f.FileInfo().Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.FileInfo().Mode())
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		rc.Close()
		out.Close()
	}
	return nil
}

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	return extractTarFromReader(tr, destDir)
}

func extractTar(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	return extractTarFromReader(tr, destDir)
}

func extractTarFromReader(tr *tar.Reader, destDir string) error {
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Clean(filepath.Join(filepath.Clean(destDir), hdr.Name))
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) &&
			target != filepath.Clean(destDir) {
			return fmt.Errorf("path traversal detected: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

type progressWriter struct {
	total   int64
	written int64
	last    time.Time
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.written += int64(n)
	now := time.Now()
	if pw.last.IsZero() || now.Sub(pw.last) >= 200*time.Millisecond || (pw.total > 0 && pw.written == pw.total) {
		pw.print()
		pw.last = now
	}
	return n, nil
}

func (pw *progressWriter) print() {
	if pw.total > 0 {
		pct := float64(pw.written) * 100.0 / float64(pw.total)
		fmt.Fprintf(os.Stderr, "\rDownloading: %s / %s (%.1f%%)", humanBytes(pw.written), humanBytes(pw.total), pct)
	} else {
		fmt.Fprintf(os.Stderr, "\rDownloading: %s", humanBytes(pw.written))
	}
}

func (pw *progressWriter) Finish() {
	pw.print()
	fmt.Fprintln(os.Stderr, "")
}

func humanBytes(n int64) string {
	f := float64(n)
	const (
		KB = 1024.0
		MB = KB * 1024.0
		GB = MB * 1024.0
	)
	switch {
	case f >= GB:
		return fmt.Sprintf("%.2f GB", f/GB)
	case f >= MB:
		return fmt.Sprintf("%.2f MB", f/MB)
	case f >= KB:
		return fmt.Sprintf("%.2f KB", f/KB)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// findHashInChecksumContent is kept for compatibility but not used.
func findHashInChecksumContent(bs []byte, assetURL string) (string, bool) {
	s := strings.ToLower(string(bs))
	var assetBase string
	if u, err := url.Parse(assetURL); err == nil {
		assetBase = strings.ToLower(filepath.Base(u.Path))
	} else {
		assetBase = strings.ToLower(filepath.Base(assetURL))
	}
	re := regexp.MustCompile(`(?i)\b([a-f0-9]{64})\b`)
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, assetBase) {
			if m := re.FindString(line); m != "" {
				return m, true
			}
		}
	}
	matches := re.FindAllString(s, -1)
	uniq := map[string]struct{}{}
	for _, m := range matches {
		uniq[m] = struct{}{}
	}
	if len(uniq) == 1 {
		for k := range uniq {
			return k, true
		}
	}
	return "", false
}
