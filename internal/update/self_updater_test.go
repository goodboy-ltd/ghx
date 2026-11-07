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
			got, err := SelfUpdater{Client: client, CurrentVersion: tc.current, Policy: activePolicy}.Update()
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
			got, err := SelfUpdater{Client: client, CurrentVersion: tc.current, Policy: activePolicy}.Update()
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
	_, err := SelfUpdater{Client: client, CurrentVersion: "0.1.9", Policy: activePolicy}.Update()
	if err == nil || client.downloads != 1 {
		t.Fatalf("expected the fake download to be attempted once, got %d downloads, %v", client.downloads, err)
	}
	if len(client.repos) != 1 || client.repos[0] != activePolicy.Repo {
		t.Fatalf("expected the policy repo to reach the client, got %v", client.repos)
	}
}

func TestSelfUpdaterRefusesWithoutNetwork(t *testing.T) {
	cases := map[string]struct {
		policy  Policy
		message string
	}{
		"off by build default": {Policy{Repo: "o/r", Reason: "build default"}, "ghx self-update is off (build default)"},
		"off by config":        {Policy{Repo: "o/r", Reason: "self_update: false"}, "ghx self-update is off (self_update: false)"},
		"homebrew":             {Policy{Repo: "o/r", Enabled: true, ManagedBy: PackageManagerHomebrew}, "ghx is installed by homebrew; run: brew upgrade ghx"},
		"asdf":                 {Policy{Repo: "o/r", Enabled: true, ManagedBy: PackageManagerAsdf}, "ghx is installed by asdf; run: asdf install ghx latest"},
		"managed and off":      {Policy{Repo: "o/r", Reason: "build default", ManagedBy: PackageManagerHomebrew}, "ghx is installed by homebrew; run: brew upgrade ghx"},
		"no repo":              {Policy{Enabled: true}, "no update repository configured"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client := &fakeReleaseClient{latest: releaseWithAssets("v9.9.9")}
			got, err := SelfUpdater{Client: client, CurrentVersion: "1.0.0", Policy: tc.policy}.Update()
			if err == nil || err.Error() != tc.message || got != "" {
				t.Fatalf("got %q, %v, want error %q", got, err, tc.message)
			}
			if len(client.repos) != 0 || client.downloads != 0 {
				t.Fatalf("expected no network, got repos %v and %d downloads", client.repos, client.downloads)
			}
		})
	}
}

func TestSelfUpdaterSkipsDevBuilds(t *testing.T) {
	client := &fakeReleaseClient{latest: releaseWithAssets("v9.9.9")}
	got, err := SelfUpdater{Client: client, CurrentVersion: devVersion, Policy: activePolicy}.Update()
	if err != nil || got != "" || len(client.repos) != 0 {
		t.Fatalf("got %q, %v, repos %v", got, err, client.repos)
	}
}

var activePolicy = Policy{Repo: "goodboy-ltd/ghx", Enabled: true}

func releaseWithAssets(tag string) release.Release {
	return release.Release{TagName: tag, Assets: []release.Asset{{Name: ghxAssetName()}, {Name: ghxChecksumsName}}}
}
