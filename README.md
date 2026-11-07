# ghx

<!-- FORK:BEGIN. Fork-only text lives between these markers, so merges from upstream leave it alone. -->

## This fork

This is [goodboy-ltd/ghx](https://github.com/goodboy-ltd/ghx), a fork of
[barnuri/ghx](https://github.com/barnuri/ghx). It tracks upstream's `master`, adds release and
security hardening, and offers each change that suits upstream back to barnuri/ghx as a pull
request. [FORK.md](FORK.md) lists what differs, the upstream pull requests and the skipped
versions. The license is upstream's MIT license, with its copyright notice kept in
[LICENSE](LICENSE).

The rest of this README is upstream's, so its install lines and links point at barnuri/ghx.
To install this fork's builds:

```sh
curl -fsSL https://raw.githubusercontent.com/goodboy-ltd/ghx/master/install.sh | sh
```

```powershell
irm https://raw.githubusercontent.com/goodboy-ltd/ghx/master/install.ps1 | iex
```

Both installers download from `goodboy-ltd/ghx` and print the version and repository before
they download anything. Two variables change that, and both are checked first:

| Variable | Default | Meaning |
|---|---|---|
| `GHX_VERSION` | latest release | Install exactly this release, such as `0.2.0` or `v0.2.0` |
| `GHX_REPO` | `goodboy-ltd/ghx` | Install from another `owner/name`, such as `barnuri/ghx` for upstream's builds |

```sh
curl -fsSL https://raw.githubusercontent.com/goodboy-ltd/ghx/master/install.sh | GHX_VERSION=0.2.0 sh
```

With Homebrew or asdf, from the first fork release (`v0.2.0`) on:

```sh
brew install goodboy-ltd/tap/ghx
asdf plugin add ghx https://github.com/goodboy-ltd/asdf-ghx.git && asdf install ghx latest
```

Fork releases are built with self-update off, so an installed `ghx` stays at its version until
you rerun the installer, `brew upgrade goodboy-ltd/tap/ghx` or `asdf install ghx latest`.

`go install github.com/goodboy-ltd/ghx@latest` does not work. The fork keeps upstream's module
path, `module github.com/barnuri/ghx` in `go.mod`, so that upstream merges stay clean, and Go
refuses a module fetched under a path other than the one it declares
([go.mod `module` directive](https://go.dev/ref/mod#go-mod-file-module)):

```text
module declares its path as: github.com/barnuri/ghx
        but was required as: github.com/goodboy-ltd/ghx
```

`go install github.com/barnuri/ghx@latest` installs upstream, not this fork. To build the fork
with Go, build from a clone. A local build reports version `dev` and never updates itself:

```sh
git clone https://github.com/goodboy-ltd/ghx.git && cd ghx
go build -o ~/.local/bin/ghx .
```

<!-- FORK:END -->

Use the right GitHub account in every folder. `ghx` is a thin wrapper around the official
[`gh`](https://cli.github.com) CLI. It picks a gh account from the directory you are in, then
runs the real `gh` with your arguments unchanged.

![ghx demo](docs/demo.gif)

- Every gh command, flag and extension works, because ghx passes them through as-is.
- gh keeps its own updates. ghx never patches or bundles it.
- macOS, Linux and Windows. One static binary, no runtime.
- Installs as `gh`, so scripts, editors and aliases use it with no changes.
- Updates itself in the background, once a day.

## Install

macOS / Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/barnuri/ghx/master/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/barnuri/ghx/master/install.ps1 | iex
```

The installer downloads the latest release for your OS and CPU, checks its sha256, and
installs `ghx` plus a `gh` shim that points to it:

| OS | Install dir | `gh` shim |
|---|---|---|
| macOS / Linux | `~/.local/bin` | symlink `gh -> ghx` |
| Windows | `%LOCALAPPDATA%\Programs\ghx` | copy `gh.exe` |

Put the install dir before the real gh in `PATH`. ghx skips its own dir when it looks for the
real gh, so the shim never calls itself. Set `GHX_SHIM=0` to install only `ghx`, or
`GHX_INSTALL_DIR` to pick another dir. On Windows, run the installer as Administrator when gh
is in the system PATH, because Windows searches the system PATH before the user PATH.

With Go: `go install github.com/barnuri/ghx@latest`, then `alias gh=ghx`.

## Configuration

1. Log in to each account once. gh stores all of them:

   ```sh
   gh auth login   # work account
   gh auth login   # personal account
   ```

2. Map folders to accounts in `~/.config/ghx/config.yaml` (Windows:
   `%APPDATA%\ghx\config.yaml`, or set `GHX_CONFIG`):

   ```yaml
   profiles:
     work:
       user: my-work-login
     personal:
       user: my-personal-login

   dirs:
     ~/code/work: work
     ~/code/personal: personal
   ```

Now every folder under `~/code/work`, at any depth, uses the work account, and every folder
under `~/code/personal` uses the personal account:

```sh
cd ~/code/work/api/src && gh pr list        # as my-work-login
cd ~/code/personal/blog && gh repo view     # as my-personal-login
```

Full example: [examples/config.yaml](examples/config.yaml).

| Key | Meaning |
|---|---|
| `profiles.<name>.user` | gh login, as shown by `gh auth status` |
| `profiles.<name>.host` | GitHub Enterprise host. Default: `github.com` |
| `profiles.<name>.name` / `.email` | git `user.name` / `user.email` for repos of this profile. See below |
| `profiles.<name>.git_identity` | `once` or `always`. See below |
| `name` / `email` / `git_identity` | Fallback for each profile that does not set the key |
| `dirs` | folder -> profile. The deepest matching folder wins |
| `dirs_follow_worktrees` | `false` stops a linked git worktree from matching `dirs` through its main checkout. Default: `true`. See below |
| `ssh_hosts` | ssh host alias in the `origin` remote URL -> profile. See below |
| `remote_user` | `true` uses the username in an https `origin` remote URL. See below |
| `default` | profile for folders outside every `dirs` entry. Default: plain gh |
| `profiles_only` | `true` refuses a `GHX_PROFILE` or `.ghx` name that is not a profile. See below |
| `manage_gh` | `true` makes ghx download gh and keep it updated. Default: use gh from `PATH` |
| `disable_updates` | `true` stops the daily ghx update check |

### Git worktrees: `dirs_follow_worktrees`

A linked worktree created outside the main checkout (`git worktree add ../repo-feature`) is not
inside any `dirs` folder. When no `dirs` entry contains the current folder, ghx checks whether it
is in a linked worktree and, if so, matches `dirs` against the worktree's main checkout instead.
A `dirs` entry that contains the current folder itself still wins.

ghx finds the main checkout by reading files, without running git: the worktree's `.git` file
names `<main>/.git/worktrees/<name>`, and that folder's `gitdir` file must point back at the
worktree. Submodules and `--separate-git-dir` repos are not followed. `gh ghx which` shows the
match as `source: <dirs folder> (via worktree main checkout <main>)`.

To turn it off:

```yaml
dirs_follow_worktrees: false
```

### Per-repo file: `.ghx`

Put a `.ghx` file in a repo to choose its profile. It holds one line: a profile name, or a gh
login if no profile has that name.

```sh
echo personal > .ghx
echo .ghx >> .git/info/exclude   # or add it to .gitignore
```

ghx uses the nearest `.ghx` in the current folder or any parent folder.

### Only your profiles: `profiles_only`

By default a `GHX_PROFILE` value or a `.ghx` name with no matching profile is used as a gh
login, so a typo or a `.ghx` committed to a repo you cloned can pick any account you are logged
into. Set `profiles_only` to allow only the profiles in your config:

```yaml
profiles_only: true
```

- A `GHX_PROFILE` or `.ghx` name that is not a profile is an error, and `gh ghx which` shows
  where the name came from. ghx does not fall back to another rule, because that would run gh
  as an account other than the one asked for.
- With `remote_user`, a login in the remote URL that matches no profile is skipped, and
  resolution goes on to `dirs` and `default`.
- `gh auth` commands that need no token (`gh auth login`, `gh auth status`, ...) still run as
  plain gh, so you can fix your setup. `gh auth git-credential` is refused like any other
  command.

### SSH host aliases: `ssh_hosts`

If you keep one ssh key per account with host aliases in `~/.ssh/config`:

```sshconfig
Host gh-work
    HostName github.com
    IdentityFile ~/.ssh/work
```

and clone with `git clone git@gh-work:org/repo.git`, map the alias to a profile:

```yaml
ssh_hosts:
  gh-work: work
  gh-personal: personal
```

ghx reads the host from `git remote get-url origin` (scp-like `git@gh-work:org/repo` or
`ssh://git@gh-work/org/repo`). It runs git only when `ssh_hosts` or `remote_user` is set.

### HTTPS remote username: `remote_user`

If you put your login in the https remote URL (it also works well with Git Credential Manager):

```sh
git clone https://my-personal-login@github.com/org/repo.git
```

turn on `remote_user`:

```yaml
remote_user: true
```

ghx uses the profile whose `user` and `host` match the URL. If no profile matches, it uses the
username as the gh login, on the URL's host.

### Git identity: `name` and `email`

ghx can set the repo-local git `user.name` and `user.email` from the matched profile. It is off
until you set `name` or `email`:

```yaml
name: My Name                # fallback for every profile
email: me@example.com
profiles:
  work:
    user: my-work-login
    email: me@work.com       # name comes from the fallback
    git_identity: always
```

Each time ghx runs in a git repo with a matched profile, it applies `git_identity`:

- `once` (default): sets a key only when the repo has no local value for it. A value you set by
  hand stays.
- `always`: replaces a local value that is different.

With no matched profile, ghx does not change git config.

### Resolution order

1. `GHX_PROFILE` environment variable
2. Nearest `.ghx` file
3. `ssh_hosts` entry for the `origin` remote's host
4. Username in the `origin` remote URL, when `remote_user` is `true`
5. Deepest matching `dirs` entry, then, in a linked git worktree, the deepest `dirs` entry that
   matches its main checkout
6. `default`
7. No match: plain gh, with your active gh account

## Commands

ghx adds one command group, `gh ghx`. Everything else goes to gh.

```text
gh ghx which     Show the profile, gh user and gh binary for the current folder
gh ghx current   Print only the profile name (empty when none), for prompts and status lines
gh ghx update    Update ghx now (and gh when manage_gh is true)
gh ghx version   Show the ghx version
```

## How it works

ghx runs `gh auth token --user <login>` and starts the real gh with `GH_TOKEN` set
(`GH_ENTERPRISE_TOKEN` and `GH_HOST` for an Enterprise host). It never runs `gh auth switch`,
so terminals in different folders do not change each other's account.

- `gh auth ...` commands run without the token, so `gh auth login` and `gh auth status` work as
  usual. `gh auth git-credential` gets the token, so git pushes as the folder's account.
- A `GH_TOKEN` (or `GH_ENTERPRISE_TOKEN`) you set yourself wins over ghx, and `gh ghx which`
  says so with a `token:` line.
- Exit codes, stdin, stdout and the TTY belong to gh. On macOS and Linux ghx replaces itself
  with gh (`exec`).

To make `git push` use the folder's account, point git at the shim:

```sh
git config --global credential.https://github.com.helper ''
git config --global --add credential.https://github.com.helper "!$HOME/.local/bin/ghx auth git-credential"
```

Use the full ghx path. VS Code and other apps started from the macOS Dock get a short PATH
without `~/.local/bin`, so a bare `gh` fails with `gh: command not found`. ghx also checks
`/opt/homebrew/bin`, `/usr/local/bin` and `/home/linuxbrew/.linuxbrew/bin` for the real gh
when PATH does not have it.

## Updates

Once a day ghx starts a background process that downloads the latest ghx release, checks its
sha256 and replaces the binary. With `manage_gh: true` it does the same for gh from
[cli/cli releases](https://github.com/cli/cli/releases). The result goes to `update.log` in the
ghx cache dir (`~/Library/Caches/ghx`, `~/.cache/ghx` or `%LOCALAPPDATA%\ghx`).
Set `GHX_NO_UPDATE=1` or `disable_updates: true` to turn it off.

## Development

```sh
go test ./...
vhs demo/demo.tape   # re-record docs/demo.gif with a stub gh and demo accounts
```

Every push to `master` runs the tests on macOS, Linux and Windows, then publishes release
`v0.1.<run>` with binaries for darwin, linux and windows on amd64 and arm64.

## License

MIT
