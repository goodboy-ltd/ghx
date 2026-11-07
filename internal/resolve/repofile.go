package resolve

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// gitEntry marks a repository's top folder. It is a directory in a normal checkout and a
// file in a linked worktree or a submodule, so any entry counts.
const gitEntry = ".git"

type repoFile struct {
	name    string
	path    string
	ignored []IgnoredFile
}

// nearestRepoFile walks from start up to the repository's top folder, or up to the home
// folder outside a repository, like git scopes .gitignore. A start outside both reads no
// .ghx. Strict also skips files someone else could have written (Unix only). Without
// strict it walks to the filesystem root and trusts every file, as ghx did before.
func nearestRepoFile(start string, strict bool) repoFile {
	stop, ok := walkBoundary(start, strict)
	if !ok {
		return repoFile{}
	}
	var found repoFile
	for dir := start; ; dir = filepath.Dir(dir) {
		file := filepath.Join(dir, RepoFileName)
		name, reason := readRepoFile(file, strict)
		if reason != "" {
			found.ignored = append(found.ignored, IgnoredFile{Path: file, Reason: reason})
		} else if name != "" {
			found.name, found.path = name, file
			return found
		}
		if dir == stop || filepath.Dir(dir) == dir {
			return found
		}
	}
}

// walkBoundary finds the last folder the walk reads. An empty stop with ok means the filesystem root.
func walkBoundary(start string, strict bool) (string, bool) {
	if !strict {
		return "", true
	}
	if top := repoToplevel(start); top != "" {
		return top, true
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", false
	}
	home = canonical(home)
	if !contains(home, start) {
		return "", false
	}
	return home, true
}

// repoToplevel finds the nearest folder holding a .git entry without running git.
func repoToplevel(start string) string {
	for dir := start; ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, gitEntry)); err == nil {
			return dir
		}
		if filepath.Dir(dir) == dir {
			return ""
		}
	}
}

// readRepoFile returns the first line that is neither blank nor a comment, or why the file
// was ignored. The trust check runs on the open file, so the file read is the file checked.
func readRepoFile(file string, strict bool) (string, string) {
	handle, err := os.Open(file)
	if err != nil {
		return "", ""
	}
	defer handle.Close()
	if strict {
		info, err := handle.Stat()
		if err != nil {
			return "", ""
		}
		if reason := untrustedReason(info); reason != "" {
			return "", reason
		}
	}
	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			return line, ""
		}
	}
	return "", ""
}
