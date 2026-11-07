package update

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

type Updater struct {
	StateDir string
	Self     SelfUpdater
	Gh       ManagedGh
	ManageGh bool
}

// Run updates ghx, then gh when ManageGh is set. When self-update is off it says so and
// carries on; when a package manager owns ghx, that is a failure naming the upgrade command.
func (updater Updater) Run(out io.Writer) error {
	return updater.run(out, true)
}

// RunQuietly is the background run. It logs instead of printing, and leaves out the ghx step
// when the policy does not let ghx replace itself, so a daily gh-only run logs no ghx refusal.
func (updater Updater) RunQuietly() {
	var log strings.Builder
	if err := updater.run(&log, updater.Self.Policy.Active()); err != nil {
		log.WriteString(err.Error())
	}
	WriteLog(updater.StateDir, strings.TrimSpace(log.String()))
}

func (updater Updater) run(out io.Writer, includeSelf bool) error {
	var failures []string
	if includeSelf {
		newVersion, err := updater.Self.Update()
		switch {
		case errors.Is(err, ErrSelfUpdateDisabled):
			fmt.Fprintln(out, err.Error())
		case err != nil:
			failures = append(failures, "ghx: "+err.Error())
		default:
			fmt.Fprintln(out, describe("ghx", newVersion))
		}
	}
	if updater.ManageGh {
		if newVersion, err := updater.Gh.Update(); err != nil {
			failures = append(failures, "gh: "+err.Error())
		} else {
			fmt.Fprintln(out, describe("gh", newVersion))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("update failed: %s", strings.Join(failures, "; "))
	}
	return nil
}

func describe(name string, newVersion string) string {
	if newVersion == "" {
		return name + " is up to date"
	}
	return name + " updated to " + newVersion
}
