package resolve

import "github.com/barnuri/ghx/internal/config"

type Match struct {
	ProfileName string
	Profile     config.Profile
	Source      string
	// Ignored lists the .ghx files the walk skipped because someone else could have written them.
	Ignored []IgnoredFile
}

type IgnoredFile struct {
	Path   string
	Reason string
}

func (match Match) Found() bool {
	return match.ProfileName != ""
}
