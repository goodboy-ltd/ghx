package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/barnuri/ghx/internal/config"
	"github.com/barnuri/ghx/internal/ghbin"
	"github.com/barnuri/ghx/internal/gitident"
	"github.com/barnuri/ghx/internal/platform"
	"github.com/barnuri/ghx/internal/proxy"
	"github.com/barnuri/ghx/internal/release"
	"github.com/barnuri/ghx/internal/resolve"
	"github.com/barnuri/ghx/internal/update"
)

// Set at build time with -ldflags "-X main.<name>=<value>". -X sets only strings.
var (
	version = "dev"
	// updateRepo is the owner/repo whose latest release ghx updates itself from.
	updateRepo = "barnuri/ghx"
	// selfUpdate is "on" or "off": whether ghx updates itself when the config has no self_update.
	selfUpdate = "on"
	// packageManager is "", "homebrew" or "asdf": the package manager that ships this build.
	packageManager = ""
)

const (
	metaCommand          = "ghx"
	backgroundUpdateFlag = "--ghx-background-update"
	profileOverrideEnv   = "GHX_PROFILE"
	noUpdateEnv          = "GHX_NO_UPDATE"
	metaUsage            = `Usage: gh ghx <command>

Commands:
  which     Show the profile, gh user and gh binary for the current directory
  current   Print only the profile name for the current directory (empty when none)
  update    Update ghx now (and gh when manage_gh is true)
  version   Show the ghx version
  help      Show this help

Every other command goes to the real gh unchanged.`
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "ghx:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return err
	}
	plan, err := updatePlan(cfg, buildVars{updateRepo, selfUpdate, packageManager}, resolvedExecutable())
	if err != nil {
		return err
	}
	stateDir := config.StateDir()
	updater := newUpdater(cfg, stateDir, plan.Self)
	if len(args) == 1 && args[0] == backgroundUpdateFlag {
		updater.RunQuietly()
		return nil
	}
	ghPath, err := locateGh(cfg, updater)
	if err != nil {
		return err
	}
	scheduleUpdate(plan, stateDir)
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	match, resolveErr := resolve.Profile(cfg, cwd, os.Getenv(profileOverrideEnv), resolve.GitOriginURL)
	applyGitIdentity(cfg, match, cwd)
	if len(args) > 0 && args[0] == metaCommand {
		return runMeta(args[1:], match, resolveErr, ghPath, updater)
	}
	warnIgnored(match.Ignored)
	// A refused profile (profiles_only) blocks only commands that need a token, so
	// gh auth login and gh auth status still run as plain gh and the user can repair the setup.
	if resolveErr != nil {
		if proxy.NeedsToken(args) {
			return resolveErr
		}
		fmt.Fprintln(os.Stderr, "ghx:", resolveErr)
	}
	env, err := buildEnv(args, match, ghPath)
	if err != nil {
		return err
	}
	exitCode, err := platform.Run(ghPath, args, env)
	if err != nil {
		return fmt.Errorf("run %s: %w", ghPath, err)
	}
	os.Exit(exitCode)
	return nil
}

func runMeta(args []string, match resolve.Match, resolveErr error, ghPath string, updater update.Updater) error {
	command := "help"
	if len(args) > 0 {
		command = args[0]
	}
	switch command {
	case "which":
		printWhich(os.Stdout, match, resolveErr, ghPath, os.Environ(), updater.Self.Policy)
	case "current":
		if resolveErr != nil {
			return resolveErr
		}
		if match.Found() {
			fmt.Println(match.ProfileName)
		}
	case "update":
		return updater.Run(os.Stdout)
	case "version":
		fmt.Printf("ghx %s\n", version)
	case "help", "--help", "-h":
		fmt.Println(metaUsage)
	default:
		return fmt.Errorf("unknown command %q. Run: gh ghx help", command)
	}
	return nil
}

type buildVars struct {
	updateRepo     string
	selfUpdate     string
	packageManager string
}

type plan struct {
	Self update.Policy
	// Background is true when the daily detached update check has anything to do.
	Background bool
}

// updatePlan combines the build-time variables, the config and the symlink-resolved
// executable path into the update policy for this run. The config's self_update and
// update_repo win over the build values. A build packageManager, or an executable inside a
// Homebrew Cellar or an asdf installs directory, means ghx never replaces itself.
func updatePlan(cfg config.Config, build buildVars, executable string) (plan, error) {
	policy := update.Policy{Repo: build.updateRepo, Reason: "build default", ManagedBy: build.packageManager}
	switch build.selfUpdate {
	case "on", "off":
		policy.Enabled = build.selfUpdate == "on"
	default:
		return plan{}, fmt.Errorf("invalid build setting main.selfUpdate=%q: want on or off", build.selfUpdate)
	}
	switch build.packageManager {
	case "", update.PackageManagerHomebrew, update.PackageManagerAsdf:
	default:
		return plan{}, fmt.Errorf("invalid build setting main.packageManager=%q: want homebrew, asdf or empty", build.packageManager)
	}
	if !config.ValidUpdateRepo(build.updateRepo) {
		return plan{}, fmt.Errorf("invalid build setting main.updateRepo=%q: want owner/repo", build.updateRepo)
	}
	if cfg.UpdateRepo != "" {
		policy.Repo = cfg.UpdateRepo
	}
	if cfg.SelfUpdate != nil {
		policy.Enabled = *cfg.SelfUpdate
		policy.Reason = fmt.Sprintf("self_update: %t", *cfg.SelfUpdate)
	}
	if policy.ManagedBy == "" {
		policy.ManagedBy = update.DetectPackageManager(executable)
	}
	return plan{Self: policy, Background: !cfg.DisableUpdates && (policy.Active() || cfg.ManageGh)}, nil
}

// resolvedExecutable returns the running binary with symlinks resolved, or "" when unknown.
func resolvedExecutable() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		return resolved
	}
	return executable
}

func newUpdater(cfg config.Config, stateDir string, policy update.Policy) update.Updater {
	client := release.NewClient()
	return update.Updater{
		StateDir: stateDir,
		Self:     update.SelfUpdater{Client: client, CurrentVersion: version, Policy: policy},
		Gh:       update.ManagedGh{Client: client, StateDir: stateDir},
		ManageGh: cfg.ManageGh,
	}
}

func locateGh(cfg config.Config, updater update.Updater) (string, error) {
	if os.Getenv("GHX_GH") != "" || !cfg.ManageGh {
		return ghbin.Locate()
	}
	if managedPath := updater.Gh.Path(); managedPath != "" {
		return managedPath, nil
	}
	fmt.Fprintln(os.Stderr, "ghx: installing gh (manage_gh: true)...")
	if _, err := updater.Gh.Update(); err != nil {
		return "", fmt.Errorf("install gh: %w", err)
	}
	return updater.Gh.Path(), nil
}

func scheduleUpdate(plan plan, stateDir string) {
	if !plan.Background || os.Getenv(noUpdateEnv) != "" || !update.IsDue(stateDir) {
		return
	}
	executable, err := os.Executable()
	if err != nil || update.MarkChecked(stateDir) != nil {
		return
	}
	_ = platform.StartDetached(executable, backgroundUpdateFlag)
}

// applyGitIdentity only warns on failure, so a git problem never blocks the gh command.
func applyGitIdentity(cfg config.Config, match resolve.Match, cwd string) {
	if !match.Found() {
		return
	}
	if err := gitident.Apply(cfg.IdentityFor(match.Profile), cwd); err != nil {
		fmt.Fprintln(os.Stderr, "ghx: set git identity:", err)
	}
}

func buildEnv(args []string, match resolve.Match, ghPath string) ([]string, error) {
	env := os.Environ()
	if !match.Found() || !proxy.NeedsToken(args) || proxy.HasExplicitToken(env) {
		return env, nil
	}
	token, err := proxy.FetchToken(ghPath, match.Profile)
	if err != nil {
		return nil, err
	}
	return proxy.WithProfile(env, match.Profile, token), nil
}

func printWhich(out io.Writer, match resolve.Match, resolveErr error, ghPath string, env []string, policy update.Policy) {
	var unknown *resolve.UnknownProfileError
	matched := false
	switch {
	case errors.As(resolveErr, &unknown):
		fmt.Fprintf(out, "profile: %s (refused: not a profile, profiles_only: true)\nsource:  %s\n",
			unknown.Name, config.AbbreviateHome(unknown.Source))
	case !match.Found():
		fmt.Fprintln(out, "profile: none (plain gh)")
	default:
		matched = true
		source := config.AbbreviateHome(match.Source)
		if match.WorktreeMain != "" {
			source += " (via worktree main checkout " + config.AbbreviateHome(match.WorktreeMain) + ")"
		}
		fmt.Fprintf(out, "profile: %s\nuser:    %s\nhost:    %s\nsource:  %s\n",
			match.ProfileName, match.Profile.User, match.Profile.HostOrDefault(), source)
	}
	for _, ignored := range match.Ignored {
		fmt.Fprintf(out, "ignored: %s (%s)\n", config.AbbreviateHome(ignored.Path), ignored.Reason)
	}
	fmt.Fprintf(out, "gh:      %s\nupdates: %s\n", config.AbbreviateHome(ghPath), policy.Describe())
	// buildEnv leaves an inherited token in place, so the profile's own token is never used.
	if name := proxy.ExplicitTokenVar(env); matched && name != "" {
		fmt.Fprintf(out, "token:   overridden by %s (unset it to use the profile's token)\n", name)
	}
}

// warnIgnored names each skipped .ghx on stderr, so a file someone else planted, or your own
// file with loose permissions, never changes or keeps the account silently.
func warnIgnored(ignored []resolve.IgnoredFile) {
	for _, file := range ignored {
		fmt.Fprintf(os.Stderr, "ghx: ignored %s: %s\n", config.AbbreviateHome(file.Path), file.Reason)
	}
}
