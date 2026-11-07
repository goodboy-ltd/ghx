package resolve

import "github.com/barnuri/ghx/internal/config"

type Match struct {
	ProfileName string
	Profile     config.Profile
	Source      string
	// WorktreeMain is the main checkout whose dirs entry matched, when cwd is a linked worktree
	// that no dirs entry contains. Empty otherwise.
	WorktreeMain string
}

func (match Match) Found() bool {
	return match.ProfileName != ""
}
