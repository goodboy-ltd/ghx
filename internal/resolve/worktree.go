package resolve

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	gitEntryName    = ".git"
	gitdirPrefix    = "gitdir:"
	worktreesDir    = "worktrees"
	backPointerName = "gitdir"
	maxLinkFileSize = 4096
)

// worktreeMainCheckout returns the main checkout of the linked git worktree that holds start,
// or "" when start is not in one. It reads files only and never runs git.
//
// A linked worktree's .git is a file holding "gitdir: <common>/worktrees/<name>", where <common>
// is the main checkout's .git directory. That directory's "gitdir" file must point back at this
// .git file, so a stray .git file cannot borrow another checkout's profile. Submodules,
// --separate-git-dir repos and bare repos have other shapes and give "".
func worktreeMainCheckout(start string) string {
	dotGit := nearestDotGitFile(start)
	if dotGit == "" {
		return ""
	}
	gitDir := linkTarget(dotGit, gitdirPrefix)
	if gitDir == "" {
		return ""
	}
	gitDir = canonical(gitDir)
	common := filepath.Dir(filepath.Dir(gitDir))
	if filepath.Base(filepath.Dir(gitDir)) != worktreesDir || filepath.Base(common) != gitEntryName {
		return ""
	}
	backPointer := linkTarget(filepath.Join(gitDir, backPointerName), "")
	if backPointer == "" || canonical(backPointer) != canonical(dotGit) {
		return ""
	}
	return filepath.Dir(common)
}

// nearestDotGitFile walks up from start to the nearest .git entry and returns its path when it is
// a file. A .git directory means a main checkout, so it gives "".
func nearestDotGitFile(start string) string {
	for dir := start; ; dir = filepath.Dir(dir) {
		entry := filepath.Join(dir, gitEntryName)
		info, err := os.Stat(entry)
		if err == nil {
			if info.Mode().IsRegular() {
				return entry
			}
			return ""
		}
		if !os.IsNotExist(err) || filepath.Dir(dir) == dir {
			return ""
		}
	}
}

// linkTarget reads the first line of file, strips prefix (which must be present when not empty)
// and resolves a relative path against the folder holding file, as git does.
func linkTarget(file string, prefix string) string {
	handle, err := os.Open(file)
	if err != nil {
		return ""
	}
	defer handle.Close()
	reader := bufio.NewReader(io.LimitReader(handle, maxLinkFileSize))
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return ""
	}
	line = strings.TrimSpace(line)
	if prefix != "" {
		rest, found := strings.CutPrefix(line, prefix)
		if !found {
			return ""
		}
		line = strings.TrimSpace(rest)
	}
	if line == "" {
		return ""
	}
	target := filepath.FromSlash(line)
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(file), target)
	}
	return target
}
