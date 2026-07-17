package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
)

func TestLatestAndDownloadVerifyDigest(t *testing.T) {
	t.Parallel()
	payload := []byte("verified executable")
	digest := sha256.Sum256(payload)
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var body string
		switch request.URL.Path {
		case "/latest":
			body = fmt.Sprintf(`{"tag_name":"v1.2.3","html_url":"https://example.test/v1.2.3","assets":[{"name":%q,"browser_download_url":"https://example.test/asset","digest":"sha256:%s"}]}`,
				AssetName(runtime.GOOS, runtime.GOARCH), hex.EncodeToString(digest[:]))
		case "/asset":
			body = string(payload)
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})

	client := Client{HTTPClient: &http.Client{Transport: transport}, APIURL: "https://example.test/latest", UserAgent: "kvmt/test"}
	release, err := client.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	downloaded, _, err := client.Download(context.Background(), release)
	if err != nil {
		t.Fatal(err)
	}
	if string(downloaded) != string(payload) {
		t.Fatalf("got payload %q, want %q", downloaded, payload)
	}
}

func TestIsNewer(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		current string
		latest  string
		want    bool
	}{
		{current: "v1.2.2", latest: "v1.2.3", want: true},
		{current: "v1.2.3", latest: "v1.2.3", want: false},
		{current: "v2.0.0", latest: "v1.9.9", want: false},
		{current: "dev", latest: "v1.0.0", want: true},
	} {
		if got := IsNewer(test.current, test.latest); got != test.want {
			t.Fatalf("IsNewer(%q, %q) = %t, want %t", test.current, test.latest, got, test.want)
		}
	}
}

func TestDownloadRejectsDigestMismatch(t *testing.T) {
	t.Parallel()
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("tampered")),
			Header:     make(http.Header),
		}, nil
	})
	client := Client{HTTPClient: &http.Client{Transport: transport}, UserAgent: "kvmt/test"}
	release := Release{TagName: "v1.2.3", Assets: []Asset{{
		Name:               AssetName(runtime.GOOS, runtime.GOARCH),
		BrowserDownloadURL: "https://example.test/asset",
		Digest:             "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	}}}
	if _, _, err := client.Download(context.Background(), release); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("got error %v, want SHA-256 mismatch", err)
	}
}

func TestDownloadDoesNotSendTokenToBrowserURL(t *testing.T) {
	t.Parallel()
	payload := []byte("verified")
	digest := sha256.Sum256(payload)
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.Header.Get("Authorization"); got != "" {
			t.Fatalf("browser download received authorisation header %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(string(payload))),
			Header:     make(http.Header),
		}, nil
	})
	client := Client{
		HTTPClient: &http.Client{Transport: transport},
		APIURL:     "https://api.github.com/repos/ThomasMcFarlane/kvmt/releases/latest",
		Token:      "secret",
		UserAgent:  "kvmt/test",
	}
	release := Release{TagName: "v1.2.3", Assets: []Asset{{
		Name:               AssetName(runtime.GOOS, runtime.GOARCH),
		BrowserDownloadURL: "https://github.com/ThomasMcFarlane/kvmt/releases/download/v1.2.3/asset",
		Digest:             "sha256:" + hex.EncodeToString(digest[:]),
	}}}
	if _, _, err := client.Download(context.Background(), release); err != nil {
		t.Fatal(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
