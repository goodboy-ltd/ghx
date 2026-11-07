package resolve

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/barnuri/ghx/internal/config"
)

// worktreeLayout writes, by hand, a main checkout at root/main and a linked worktree "feat"
// outside it at root/feat, as "git worktree add ../feat" would leave them.
type worktreeLayout struct {
	root, main, worktree string
}

func newWorktreeLayout(t *testing.T) worktreeLayout {
	t.Helper()
	root := t.TempDir()
	layout := worktreeLayout{root: root, main: filepath.Join(root, "main"), worktree: filepath.Join(root, "feat")}
	mkdirs(t, filepath.Join(layout.main, ".git", "worktrees", "feat"), filepath.Join(layout.worktree, "sub"))
	return layout
}

func (layout worktreeLayout) gitDir() string {
	return filepath.Join(layout.main, ".git", "worktrees", "feat")
}

func (layout worktreeLayout) link(t *testing.T, dotGit string, backPointer string) {
	t.Helper()
	writeFile(t, filepath.Join(layout.worktree, ".git"), dotGit)
	if backPointer != "" {
		writeFile(t, filepath.Join(layout.gitDir(), "gitdir"), backPointer)
	}
}

func (layout worktreeLayout) config(extra map[string]string) config.Config {
	dirs := map[string]string{layout.main: "work"}
	for dir, profile := range extra {
		dirs[dir] = profile
	}
	return config.Config{
		Profiles: map[string]config.Profile{"work": {User: "me-work"}, "personal": {User: "me"}},
		Dirs:     dirs,
	}
}

func mkdirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func boolPointer(value bool) *bool { return &value }

func TestWorktreeMatchesMainCheckoutDirs(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(t *testing.T, layout worktreeLayout) config.Config
		want      string
		wantViaWt bool
	}{
		{
			name: "absolute gitdir",
			setup: func(t *testing.T, layout worktreeLayout) config.Config {
				layout.link(t, "gitdir: "+layout.gitDir()+"\n", filepath.Join(layout.worktree, ".git")+"\n")
				return layout.config(nil)
			},
			want: "work", wantViaWt: true,
		},
		{
			name: "relative gitdir and back-pointer",
			setup: func(t *testing.T, layout worktreeLayout) config.Config {
				layout.link(t, "gitdir: ../main/.git/worktrees/feat\n", "../../../../feat/.git\n")
				return layout.config(nil)
			},
			want: "work", wantViaWt: true,
		},
		{
			name: "forward slashes and CRLF",
			setup: func(t *testing.T, layout worktreeLayout) config.Config {
				layout.link(t, "gitdir: "+filepath.ToSlash(layout.gitDir())+"\r\n", filepath.ToSlash(filepath.Join(layout.worktree, ".git"))+"\r\n")
				return layout.config(nil)
			},
			want: "work", wantViaWt: true,
		},
		{
			name: "own dirs key wins over main checkout",
			setup: func(t *testing.T, layout worktreeLayout) config.Config {
				layout.link(t, "gitdir: "+layout.gitDir()+"\n", filepath.Join(layout.worktree, ".git")+"\n")
				return layout.config(map[string]string{layout.worktree: "personal"})
			},
			want: "personal",
		},
		{
			name: "submodule gitdir adds nothing",
			setup: func(t *testing.T, layout worktreeLayout) config.Config {
				modules := filepath.Join(layout.main, ".git", "modules", "x")
				mkdirs(t, modules)
				writeFile(t, filepath.Join(layout.worktree, ".git"), "gitdir: ../main/.git/modules/x\n")
				writeFile(t, filepath.Join(modules, "gitdir"), filepath.Join(layout.worktree, ".git")+"\n")
				return layout.config(nil)
			},
		},
		{
			name: "separate git dir adds nothing",
			setup: func(t *testing.T, layout worktreeLayout) config.Config {
				writeFile(t, filepath.Join(layout.worktree, ".git"), "gitdir: "+filepath.Join(layout.main, ".git")+"\n")
				return layout.config(nil)
			},
		},
		{
			name: "missing back-pointer adds nothing",
			setup: func(t *testing.T, layout worktreeLayout) config.Config {
				layout.link(t, "gitdir: "+layout.gitDir()+"\n", "")
				return layout.config(nil)
			},
		},
		{
			name: "mismatched back-pointer adds nothing",
			setup: func(t *testing.T, layout worktreeLayout) config.Config {
				other := filepath.Join(layout.root, "other")
				mkdirs(t, other)
				writeFile(t, filepath.Join(other, ".git"), "gitdir: "+layout.gitDir()+"\n")
				layout.link(t, "gitdir: "+layout.gitDir()+"\n", filepath.Join(other, ".git")+"\n")
				return layout.config(nil)
			},
		},
		{
			name: "dot git file without gitdir line adds nothing",
			setup: func(t *testing.T, layout worktreeLayout) config.Config {
				layout.link(t, "not a link\n", filepath.Join(layout.worktree, ".git")+"\n")
				return layout.config(nil)
			},
		},
		{
			name: "opt-out disables the fallback",
			setup: func(t *testing.T, layout worktreeLayout) config.Config {
				layout.link(t, "gitdir: "+layout.gitDir()+"\n", filepath.Join(layout.worktree, ".git")+"\n")
				cfg := layout.config(nil)
				cfg.DirsFollowWorktrees = boolPointer(false)
				return cfg
			},
		},
		{
			name: "explicit opt-in",
			setup: func(t *testing.T, layout worktreeLayout) config.Config {
				layout.link(t, "gitdir: "+layout.gitDir()+"\n", filepath.Join(layout.worktree, ".git")+"\n")
				cfg := layout.config(nil)
				cfg.DirsFollowWorktrees = boolPointer(true)
				return cfg
			},
			want: "work", wantViaWt: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			layout := newWorktreeLayout(t)
			cfg := tc.setup(t, layout)
			got := Profile(cfg, filepath.Join(layout.worktree, "sub"), "", noRemote)
			if got.ProfileName != tc.want {
				t.Fatalf("got %+v, want profile %q", got, tc.want)
			}
			wantMain := ""
			if tc.wantViaWt {
				wantMain = canonical(layout.main)
			}
			if got.WorktreeMain != wantMain {
				t.Fatalf("WorktreeMain = %q, want %q", got.WorktreeMain, wantMain)
			}
			if tc.wantViaWt && got.Source != canonical(layout.main) {
				t.Fatalf("Source = %q, want the main checkout's dirs key %q", got.Source, canonical(layout.main))
			}
		})
	}
}

func TestWorktreeMainReachedThroughSymlinkMatchesCanonicalKey(t *testing.T) {
	layout := newWorktreeLayout(t)
	link := filepath.Join(layout.root, "main-link")
	if err := os.Symlink(layout.main, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	viaLink := filepath.Join(link, ".git", "worktrees", "feat")
	layout.link(t, "gitdir: "+viaLink+"\n", filepath.Join(layout.worktree, ".git")+"\n")
	cfg := layout.config(nil)
	cfg.Dirs = map[string]string{canonical(layout.main): "work"}
	got := Profile(cfg, layout.worktree, "", noRemote)
	if got.ProfileName != "work" || got.WorktreeMain != canonical(layout.main) {
		t.Fatalf("unexpected match %+v", got)
	}
}

func TestMainCheckoutHasNoWorktreeSource(t *testing.T) {
	layout := newWorktreeLayout(t)
	layout.link(t, "gitdir: "+layout.gitDir()+"\n", filepath.Join(layout.worktree, ".git")+"\n")
	got := Profile(layout.config(nil), layout.main, "", noRemote)
	if got.ProfileName != "work" || got.WorktreeMain != "" {
		t.Fatalf("unexpected match %+v", got)
	}
}

func TestRepoFileBeatsWorktreeDirs(t *testing.T) {
	layout := newWorktreeLayout(t)
	layout.link(t, "gitdir: "+layout.gitDir()+"\n", filepath.Join(layout.worktree, ".git")+"\n")
	writeFile(t, filepath.Join(layout.worktree, RepoFileName), "personal\n")
	if got := Profile(layout.config(nil), filepath.Join(layout.worktree, "sub"), "", noRemote); got.ProfileName != "personal" {
		t.Fatalf("got %+v, want personal", got)
	}
}

func TestWorktreeLookupRunsNoSubprocess(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "worktree.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if path == "os/exec" || path == "syscall" {
			t.Fatalf("worktree.go imports %s; the worktree lookup must read files only", path)
		}
	}
}
