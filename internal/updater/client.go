package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const latestReleaseURL = "https://api.github.com/repos/ThomasMcFarlane/kvmt/releases/latest"

type Client struct {
	HTTPClient *http.Client
	APIURL     string
	Token      string
	UserAgent  string
}

type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	APIURL             string `json:"url"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

func NewClient(version, token string) Client {
	return Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		APIURL:     latestReleaseURL,
		Token:      token,
		UserAgent:  "kvmt/" + version,
	}
}

func (client Client) Latest(ctx context.Context) (Release, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.APIURL, nil)
	if err != nil {
		return Release{}, fmt.Errorf("create GitHub release request: %w", err)
	}
	client.setHeaders(request)
	response, err := client.httpClient().Do(request)
	if err != nil {
		return Release{}, fmt.Errorf("check GitHub releases: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return Release{}, fmt.Errorf("GitHub release repository is not public yet (HTTP 404)")
	}
	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("check GitHub releases: HTTP %s", response.Status)
	}
	var release Release
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&release); err != nil {
		return Release{}, fmt.Errorf("decode GitHub release: %w", err)
	}
	if release.TagName == "" {
		return Release{}, fmt.Errorf("GitHub returned a release without a version")
	}
	return release, nil
}

func (client Client) Download(ctx context.Context, release Release) ([]byte, Asset, error) {
	name := AssetName(runtime.GOOS, runtime.GOARCH)
	var selected Asset
	for _, asset := range release.Assets {
		if asset.Name == name {
			selected = asset
			break
		}
	}
	if selected.Name == "" {
		return nil, Asset{}, fmt.Errorf("release %s has no asset for %s/%s", release.TagName, runtime.GOOS, runtime.GOARCH)
	}
	if !strings.HasPrefix(selected.Digest, "sha256:") {
		return nil, Asset{}, fmt.Errorf("release asset %s has no SHA-256 digest", selected.Name)
	}
	downloadURL := selected.BrowserDownloadURL
	privateAsset := client.Token != "" && selected.APIURL != ""
	if privateAsset {
		downloadURL = selected.APIURL
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, Asset{}, fmt.Errorf("create release download request: %w", err)
	}
	client.setHeaders(request)
	if privateAsset {
		request.Header.Set("Accept", "application/octet-stream")
	}
	response, err := client.httpClient().Do(request)
	if err != nil {
		return nil, Asset{}, fmt.Errorf("download %s: %w", selected.Name, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, Asset{}, fmt.Errorf("download %s: HTTP %s", selected.Name, response.Status)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 100<<20))
	if err != nil {
		return nil, Asset{}, fmt.Errorf("download %s: %w", selected.Name, err)
	}
	digest := sha256.Sum256(payload)
	actual := hex.EncodeToString(digest[:])
	expected := strings.TrimPrefix(selected.Digest, "sha256:")
	if !strings.EqualFold(actual, expected) {
		return nil, Asset{}, fmt.Errorf("refuse update: SHA-256 mismatch for %s", selected.Name)
	}
	return payload, selected, nil
}

func AssetName(goos, goarch string) string {
	name := fmt.Sprintf("kvmt-%s-%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func IsNewer(current, latest string) bool {
	currentParts, currentOK := parseVersion(current)
	latestParts, latestOK := parseVersion(latest)
	if !latestOK {
		return false
	}
	if !currentOK {
		return true
	}
	for i := range latestParts {
		if latestParts[i] != currentParts[i] {
			return latestParts[i] > currentParts[i]
		}
	}
	return false
}

func parseVersion(value string) ([3]int, bool) {
	var result [3]int
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(value), "v"), ".")
	if len(parts) != len(result) {
		return result, false
	}
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return result, false
		}
		result[i] = value
	}
	return result, true
}

func (client Client) setHeaders(request *http.Request) {
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", client.UserAgent)
	apiRequest, _ := http.NewRequest(http.MethodGet, client.APIURL, nil)
	sameAPIHost := apiRequest != nil && request.URL.Hostname() == apiRequest.URL.Hostname()
	if client.Token != "" && sameAPIHost {
		request.Header.Set("Authorization", "Bearer "+client.Token)
	}
}

func (client Client) httpClient() *http.Client {
	if client.HTTPClient != nil {
		return client.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}
