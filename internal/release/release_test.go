package release

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLatestSelectsNewestCompleteStableRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[
			{"tag_name":"controller-v0.2.26","assets":[{"name":"ClubPay-Controller-win-x64.zip","browser_download_url":"https://example/26.zip"}]},
			{"tag_name":"controller-v0.2.25","assets":[{"name":"ClubPay-Controller-win-x64.zip","browser_download_url":"https://example/25.zip"},{"name":"ClubPay-Controller-win-x64.zip.sha256","browser_download_url":"https://example/25.sha"}]},
			{"tag_name":"controller-v0.2.24","assets":[{"name":"ClubPay-Controller-win-x64.zip","browser_download_url":"https://example/24.zip"},{"name":"ClubPay-Controller-win-x64.zip.sha256","browser_download_url":"https://example/24.sha"}]}
		]`))
	}))
	defer server.Close()

	got, err := Latest(context.Background(), server.Client(), server.URL, "controller-v", "ClubPay-Controller-win-x64.zip")
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "controller-v0.2.25" || got.DownloadURL != "https://example/25.zip" || got.ChecksumURL != "https://example/25.sha" {
		t.Fatalf("unexpected artifact: %#v", got)
	}
}

func TestCompareVersions(t *testing.T) {
	if CompareVersions("controller-v0.2.26", "controller-v0.2.9") <= 0 {
		t.Fatal("expected newer controller release")
	}
	if CompareVersions("v0.4.25", "v0.4.25") != 0 {
		t.Fatal("expected equality")
	}
	if CompareVersions("v0.4.24", "v0.4.25") >= 0 {
		t.Fatal("expected older agent release")
	}
}
