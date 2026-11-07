//go:build unix

package resolve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func chmod(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func TestWritableRepoFileIsIgnored(t *testing.T) {
	cases := []struct {
		name       string
		mode       os.FileMode
		wantReason string
	}{
		{"group-writable", 0o664, "group-writable, mode 0664"},
		{"world-writable", 0o646, "world-writable, mode 0646"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, cfg := newTree(t)
			loose := filepath.Join(root, "work/api", RepoFileName)
			writeFile(t, loose, "personal\n")
			chmod(t, loose, tc.mode)
			got := mustProfile(t, cfg, filepath.Join(root, "work/api/src"), "", noRemote)
			if got.ProfileName != "work" {
				t.Fatalf("got %+v, want the dirs profile work", got)
			}
			want := []IgnoredFile{{Path: canonical(loose), Reason: tc.wantReason}}
			if len(got.Ignored) != 1 || got.Ignored[0] != want[0] {
				t.Fatalf("got ignored %+v, want %+v", got.Ignored, want)
			}
		})
	}
}

func TestIgnoredRepoFileFallsBackToTrustedParent(t *testing.T) {
	root, cfg := newTree(t)
	loose := filepath.Join(root, "work/api", RepoFileName)
	writeFile(t, loose, "work\n")
	chmod(t, loose, 0o666)
	writeFile(t, filepath.Join(root, "work", RepoFileName), "personal\n")
	got := mustProfile(t, cfg, filepath.Join(root, "work/api/src"), "", noRemote)
	if got.ProfileName != "personal" || len(got.Ignored) != 1 || got.Ignored[0].Path != canonical(loose) {
		t.Fatalf("unexpected match %+v", got)
	}
}

func TestLenientReadsWritableRepoFile(t *testing.T) {
	root, cfg := newTree(t)
	loose := filepath.Join(root, "work/api", RepoFileName)
	writeFile(t, loose, "personal\n")
	chmod(t, loose, 0o666)
	lenient := false
	cfg.GhxFileStrict = &lenient
	got := mustProfile(t, cfg, filepath.Join(root, "work/api/src"), "", noRemote)
	if got.ProfileName != "personal" || len(got.Ignored) != 0 {
		t.Fatalf("unexpected match %+v", got)
	}
}

func TestOwnershipReason(t *testing.T) {
	file := filepath.Join(t.TempDir(), RepoFileName)
	writeFile(t, file, "work\n")
	chmod(t, file, 0o644)
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if reason := ownershipReason(info, os.Getuid()); reason != "" {
		t.Fatalf("own 0644 file rejected: %s", reason)
	}
	if reason := ownershipReason(info, os.Getuid()+1); !strings.HasPrefix(reason, "owned by uid ") {
		t.Fatalf("file owned by another uid accepted: %q", reason)
	}
}
