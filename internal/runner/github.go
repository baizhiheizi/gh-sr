package runner

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type GitHubClient struct {
	pat    string
	client *http.Client
}

type GitHubRunner struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	OS     string   `json:"os"`
	Status string   `json:"status"`
	Busy   bool     `json:"busy"`
	Labels []Label  `json:"labels"`
}

type Label struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type runnersResponse struct {
	TotalCount int            `json:"total_count"`
	Runners    []GitHubRunner `json:"runners"`
}

type tokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

type releaseResponse struct {
	TagName string `json:"tag_name"`
}

func NewGitHubClient(pat string) *GitHubClient {
	return &GitHubClient{
		pat:    pat,
		client: &http.Client{},
	}
}

func (g *GitHubClient) GetRegistrationToken(repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/actions/runners/registration-token", repo)
	resp, err := g.post(url, nil)
	if err != nil {
		return "", fmt.Errorf("getting registration token for %s: %w", repo, err)
	}

	var tok tokenResponse
	if err := json.Unmarshal(resp, &tok); err != nil {
		return "", fmt.Errorf("parsing registration token: %w", err)
	}
	if tok.Token == "" {
		return "", fmt.Errorf("empty registration token for %s (check PAT permissions)", repo)
	}
	return tok.Token, nil
}

func (g *GitHubClient) GetRemovalToken(repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/actions/runners/remove-token", repo)
	resp, err := g.post(url, nil)
	if err != nil {
		return "", fmt.Errorf("getting removal token for %s: %w", repo, err)
	}

	var tok tokenResponse
	if err := json.Unmarshal(resp, &tok); err != nil {
		return "", fmt.Errorf("parsing removal token: %w", err)
	}
	if tok.Token == "" {
		return "", fmt.Errorf("empty removal token for %s", repo)
	}
	return tok.Token, nil
}

func (g *GitHubClient) ListRunners(repo string) ([]GitHubRunner, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/actions/runners", repo)
	resp, err := g.get(url)
	if err != nil {
		return nil, fmt.Errorf("listing runners for %s: %w", repo, err)
	}

	var rr runnersResponse
	if err := json.Unmarshal(resp, &rr); err != nil {
		return nil, fmt.Errorf("parsing runners list: %w", err)
	}
	return rr.Runners, nil
}

func (g *GitHubClient) DeleteRunner(repo string, runnerID int64) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/actions/runners/%d", repo, runnerID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	g.setHeaders(req)

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("deleting runner %d: %w", runnerID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete runner %d: HTTP %d: %s", runnerID, resp.StatusCode, body)
	}
	return nil
}

func (g *GitHubClient) GetLatestRunnerVersion() (string, error) {
	url := "https://api.github.com/repos/actions/runner/releases/latest"
	resp, err := g.get(url)
	if err != nil {
		return "", fmt.Errorf("fetching latest runner version: %w", err)
	}

	var rel releaseResponse
	if err := json.Unmarshal(resp, &rel); err != nil {
		return "", fmt.Errorf("parsing release: %w", err)
	}

	version := strings.TrimPrefix(rel.TagName, "v")
	if version == "" {
		return "", fmt.Errorf("empty version from GitHub releases API")
	}
	return version, nil
}

func (g *GitHubClient) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+g.pat)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}

func (g *GitHubClient) get(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	g.setHeaders(req)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	return body, nil
}

func (g *GitHubClient) post(url string, payload io.Reader) ([]byte, error) {
	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return nil, err
	}
	g.setHeaders(req)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	return body, nil
}
