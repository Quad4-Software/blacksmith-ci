package update

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"codefloe.com/crowci/crow/v6/version"
)

func TestCheckForUpdate(t *testing.T) {
	version.Version = "1.0.0"
	fixtureHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/versions.json" {
			http.NotFound(w, r)
			return
		}

		// Return a JSON array of VersionEntry objects
		_, _ = io.WriteString(w, `[
			{"version": "1.0.1", "title": "Release 1.0.1", "aliases": ["latest"]},
			{"version": "1.0.2", "title": "Release 1.0.2", "aliases": ["dev"]},
			{"version": "1.0.3", "title": "Release 1.0.3", "aliases": ["rc"]}
		]`)
	}
	ts := httptest.NewServer(http.HandlerFunc(fixtureHandler))
	defer ts.Close()

	newVersion, err := checkForUpdate(t.Context(), ts.URL+"/versions.json", false)
	if err != nil {
		t.Fatalf("Failed to check for updates: %v", err)
	}

	if newVersion == nil || newVersion.Version != "1.0.1" {
		t.Fatalf("Expected a new version 1.0.1, got: %s", newVersion)
	}
}

func TestDownloadNewVersion(t *testing.T) {
	downloadFilePath := "/crow-cli_linux_amd64.tar.gz"

	fixtureHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != downloadFilePath {
			http.NotFound(w, r)
			return
		}

		_, _ = io.WriteString(w, `blob`)
	}
	ts := httptest.NewServer(http.HandlerFunc(fixtureHandler))
	defer ts.Close()

	file, err := downloadNewVersion(t.Context(), ts.URL+downloadFilePath)
	if err != nil {
		t.Fatalf("Failed to download new version: %v", err)
	}

	if file == "" {
		t.Fatalf("Expected a file path, got: %s", file)
	}

	_ = os.Remove(file)
}
