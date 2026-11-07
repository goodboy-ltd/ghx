package update

import (
	"errors"
	"strings"
	"testing"

	"github.com/barnuri/ghx/internal/release"
)

type fakeReleaseClient struct {
	latest    release.Release
	repos     []string
	downloads int
}

func (client *fakeReleaseClient) Latest(repo string) (release.Release, error) {
	client.repos = append(client.repos, repo)
	return client.latest, nil
}

func (client *fakeReleaseClient) DownloadVerified(release.Asset, release.Asset) ([]byte, error) {
	client.downloads++
	return nil, errors.New("fake client: no downloads")
}

func TestSelfUpdaterSkipsNonNewerReleases(t *testing.T) {
	cases := map[string]struct{ current, latestTag string }{
		"same version":           {"0.1.9", "v0.1.9"},
		"latest is lower":        {"1.0.0", "v0.1.9"},
		"latest is a prerelease": {"1.0.0", "v1.0.0-rc.1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client := &fakeReleaseClient{latest: releaseWithAssets(tc.latestTag)}
			got, err := SelfUpdater{Client: client, CurrentVersion: tc.current}.Update()
			if err != nil || got != "" {
				t.Fatalf("got %q, %v", got, err)
			}
			if client.downloads != 0 {
				t.Fatalf("expected no download, got %d", client.downloads)
			}
		})
	}
}

func TestSelfUpdaterRejectsUnparsableVersions(t *testing.T) {
	cases := map[string]struct{ current, latestTag string }{
		"unparsable latest":  {"1.0.0", "nightly"},
		"unparsable current": {"local-build", "v1.0.0"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client := &fakeReleaseClient{latest: releaseWithAssets(tc.latestTag)}
			got, err := SelfUpdater{Client: client, CurrentVersion: tc.current}.Update()
			if err == nil || !strings.Contains(err.Error(), "invalid version") || got != "" {
				t.Fatalf("got %q, %v", got, err)
			}
			if client.downloads != 0 {
				t.Fatalf("expected no download, got %d", client.downloads)
			}
		})
	}
}

func TestSelfUpdaterDownloadsNewerRelease(t *testing.T) {
	client := &fakeReleaseClient{latest: releaseWithAssets("v0.2.0")}
	_, err := SelfUpdater{Client: client, CurrentVersion: "0.1.9"}.Update()
	if err == nil || client.downloads != 1 {
		t.Fatalf("expected the fake download to be attempted once, got %d downloads, %v", client.downloads, err)
	}
}

func TestSelfUpdaterSkipsDevBuilds(t *testing.T) {
	client := &fakeReleaseClient{latest: releaseWithAssets("v9.9.9")}
	got, err := SelfUpdater{Client: client, CurrentVersion: devVersion}.Update()
	if err != nil || got != "" || len(client.repos) != 0 {
		t.Fatalf("got %q, %v, repos %v", got, err, client.repos)
	}
}

func releaseWithAssets(tag string) release.Release {
	return release.Release{TagName: tag, Assets: []release.Asset{{Name: ghxAssetName()}, {Name: ghxChecksumsName}}}
}
