package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/anomalyco/mihomo-manager/internal/domain"
)

// ReleaseRepo provides GitHub release operations.
type ReleaseRepo interface {
	Download(ctx context.Context, url, dest string) error
	ListVersions(ctx context.Context, owner, repo string, limit int) ([]domain.VersionInfo, error)
	LatestVersion(ctx context.Context, owner, repo string) (string, error)
}

// GitHubRepo implements ReleaseRepo using the GitHub API.
type GitHubRepo struct{}

// NewGitHubRepo returns a new GitHubRepo.
func NewGitHubRepo() *GitHubRepo { return &GitHubRepo{} }

func (GitHubRepo) Download(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	resp, err := getDownloadClient().Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: status %d", url, resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", dest, err)
	}
	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dest, err)
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(dest)
		return fmt.Errorf("writing %s: %w", dest, err)
	}
	return nil
}

func (GitHubRepo) ListVersions(ctx context.Context, owner, repo string, limit int) ([]domain.VersionInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=%d", owner, repo, limit)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := getDownloadClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API: status %d", resp.StatusCode)
	}
	var releases []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decoding releases: %w", err)
	}
	versions := make([]domain.VersionInfo, 0, len(releases))
	for _, r := range releases {
		versions = append(versions, domain.VersionInfo{Tag: r.TagName})
	}
	return versions, nil
}

func (GitHubRepo) LatestVersion(ctx context.Context, owner, repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := getDownloadClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching latest release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API: status %d", resp.StatusCode)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("decoding release: %w", err)
	}
	return release.TagName, nil
}
