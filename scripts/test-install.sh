#!/bin/sh
# Tests install.sh against a fake release served from a local directory.
#
#   sh scripts/test-install.sh
#
# Nothing touches the network: each run of install.sh gets a PATH holding only a
# fake uname, a fake curl or wget that serves https://github.com/... from the
# fixture directory, and links to the few system tools the installer needs.
# Everything is written under one temporary directory ($RUNNER_TEMP in CI, else
# $TMPDIR), which is removed on exit. TEST_INSTALL_SH picks the shell that runs
# install.sh (default: sh), e.g. TEST_INSTALL_SH=dash.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
installer="$root/install.sh"
run_sh=$(command -v "${TEST_INSTALL_SH:-sh}") || {
  echo "test-install: shell ${TEST_INSTALL_SH:-sh} not found" >&2
  exit 1
}

work=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/ghx-test-install.XXXXXX")
trap 'rm -rf "$work"' EXIT
releases="$work/releases"

host_sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$@"
  else
    shasum -a 256 "$@"
  fi
}

# --- fixture releases --------------------------------------------------------

# make_release <dir> <version>: the four raw assets plus checksums.txt. Each asset
# is a script that prints its version and target, so a test can tell which one
# was installed.
make_release() {
  mkdir -p "$1"
  for target in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64; do
    printf '#!/bin/sh\necho "ghx %s %s"\n' "$2" "$target" > "$1/ghx_$target"
  done
  (cd "$1" && host_sha256 ghx_* > checksums.txt)
}

gb="$releases/goodboy-ltd/ghx/releases"
make_release "$gb/latest/download" 0.2.1
make_release "$gb/download/v0.2.0" 0.2.0
make_release "$gb/download/v0.3.0-rc.1" 0.3.0-rc.1
make_release "$releases/example-org/ghx-fork/releases/latest/download" 0.9.0

# v0.0.1: every asset differs from the one checksums.txt describes.
make_release "$gb/download/v0.0.1" 0.0.1
for asset in "$gb"/download/v0.0.1/ghx_*; do
  echo 'echo tampered' >> "$asset"
done

# v0.0.2: checksums.txt has no line for the linux assets.
make_release "$gb/download/v0.0.2" 0.0.2
grep -v linux "$gb/download/v0.0.2/checksums.txt" > "$work/checksums.tmp"
mv "$work/checksums.tmp" "$gb/download/v0.0.2/checksums.txt"

# --- the PATH install.sh runs with --------------------------------------------

# link_tools <dir> <tool...>: links each tool the host has into <dir>.
link_tools() {
  dir=$1
  shift
  mkdir -p "$dir"
  for tool in "$@"; do
    if path=$(command -v "$tool" 2>/dev/null); then
      ln -s "$path" "$dir/$tool"
    fi
  done
}

base_tools="awk chmod cp cut ln mkdir mktemp mv rm"
# shellcheck disable=SC2086 # word splitting of the tool list is intended
link_tools "$work/sys" $base_tools sha256sum shasum
# shellcheck disable=SC2086
link_tools "$work/sys-shasum" $base_tools shasum

fake="$work/fake"
mkdir -p "$fake/curl" "$fake/wget" "$fake/none"

cat > "$fake/none/uname" <<'EOF'
#!/bin/sh
case $1 in
  -s) echo "$FAKE_UNAME_S" ;;
  -m) echo "$FAKE_UNAME_M" ;;
  *) exit 2 ;;
esac
EOF

# serve <tool> <url> <out>: copies the fixture file for a github.com URL.
serve='
echo "$tool $url" >> "$FAKE_LOG"
case $url in
  https://github.com/*) path=${url#https://github.com/} ;;
  *) exit 6 ;;
esac
[ -f "$FAKE_RELEASES/$path" ] || exit 22
cp "$FAKE_RELEASES/$path" "$out"
'

cat > "$fake/curl/curl" <<EOF
#!/bin/sh
tool=curl url= out=
while [ \$# -gt 0 ]; do
  case \$1 in
    -o) out=\$2; shift 2 ;;
    -*) shift ;;
    *) url=\$1; shift ;;
  esac
done
$serve
EOF

cat > "$fake/wget/wget" <<EOF
#!/bin/sh
tool=wget url= out=
while [ \$# -gt 0 ]; do
  case \$1 in
    -qO) out=\$2; shift 2 ;;
    -*) shift ;;
    *) url=\$1; shift ;;
  esac
done
$serve
EOF

cp "$fake/none/uname" "$fake/curl/uname"
cp "$fake/none/uname" "$fake/wget/uname"
chmod 755 "$fake"/*/*

# --- harness -----------------------------------------------------------------

cases=0
failed=0
case_ok=1

# begin <name>: starts a case with a fresh HOME and request log.
begin() {
  cases=$((cases + 1))
  name=$1
  case_ok=1
  dir="$work/case$cases"
  home="$dir/home"
  log="$dir/requests"
  out="$dir/output"
  mkdir -p "$home" "$dir/tmp"
  : > "$log"
  bins="$fake/curl"
  sys="$work/sys"
  uname_s=Linux
  uname_m=x86_64
}

# install [VAR=value...]: runs install.sh with only these variables set.
install() {
  set +e
  env -i HOME="$home" PATH="$bins:$sys" TMPDIR="$dir/tmp" \
    FAKE_RELEASES="$releases" FAKE_LOG="$log" \
    FAKE_UNAME_S="$uname_s" FAKE_UNAME_M="$uname_m" \
    "$@" "$run_sh" "$installer" > "$out" 2>&1
  status=$?
  set -e
}

problem() {
  echo "  - $*"
  case_ok=0
}

finish() {
  if [ -n "$(ls -A "$dir/tmp")" ]; then
    problem "install.sh left files in its temp dir"
  fi
  if [ "$case_ok" = 1 ]; then
    echo "ok   $name"
  else
    echo "FAIL $name"
    echo "  install.sh output:"
    sed 's/^/  | /' "$out"
    failed=$((failed + 1))
  fi
}

expect_success() {
  [ "$status" = 0 ] || problem "exit status $status, want 0"
}

expect_failure() {
  [ "$status" != 0 ] || problem "exit status 0, want a failure"
}

expect_output() {
  grep -F -- "$1" "$out" >/dev/null || problem "output lacks: $1"
}

# expect_requests <line...>: the exact downloads, in order.
expect_requests() {
  printf '%s\n' "$@" > "$dir/want-requests"
  if ! cmp -s "$dir/want-requests" "$log"; then
    problem "requests differ, want:"
    sed 's/^/      /' "$dir/want-requests"
    echo "    got:"
    sed 's/^/      /' "$log"
  fi
}

expect_no_requests() {
  [ ! -s "$log" ] || problem "downloaded before failing: $(cat "$log")"
}

# expect_installed <bin dir> <version> <target>
expect_installed() {
  if [ ! -x "$1/ghx" ]; then
    problem "$1/ghx is not installed"
    return
  fi
  got=$("$1/ghx")
  [ "$got" = "ghx $2 $3" ] || problem "installed binary prints '$got', want 'ghx $2 $3'"
}

expect_shim() {
  [ -L "$1/gh" ] && [ "$(readlink "$1/gh")" = ghx ] || problem "$1/gh is not a symlink to ghx"
}

expect_absent() {
  [ ! -e "$1" ] && [ ! -L "$1" ] || problem "$1 exists"
}

latest="https://github.com/goodboy-ltd/ghx/releases/latest/download"
pinned="https://github.com/goodboy-ltd/ghx/releases/download"

# --- cases -------------------------------------------------------------------

begin "no variables: latest release of goodboy-ltd/ghx, with the gh shim"
install
expect_success
expect_output "Installing ghx latest from goodboy-ltd/ghx"
expect_requests "curl $latest/ghx_linux_amd64" "curl $latest/checksums.txt"
expect_installed "$home/.local/bin" 0.2.1 linux_amd64
expect_shim "$home/.local/bin"
finish

begin "empty GHX_REPO and GHX_VERSION fall back to the defaults"
install GHX_REPO= GHX_VERSION=
expect_success
expect_output "Installing ghx latest from goodboy-ltd/ghx"
expect_installed "$home/.local/bin" 0.2.1 linux_amd64
finish

begin "GHX_VERSION=0.2.0 installs exactly v0.2.0 (darwin arm64)"
uname_s=Darwin uname_m=arm64
install GHX_VERSION=0.2.0
expect_success
expect_output "Installing ghx v0.2.0 from goodboy-ltd/ghx"
expect_requests "curl $pinned/v0.2.0/ghx_darwin_arm64" "curl $pinned/v0.2.0/checksums.txt"
expect_installed "$home/.local/bin" 0.2.0 darwin_arm64
finish

begin "GHX_VERSION=v0.3.0-rc.1 takes a leading v and a prerelease (linux aarch64)"
uname_m=aarch64
install GHX_VERSION=v0.3.0-rc.1
expect_success
expect_output "Installing ghx v0.3.0-rc.1 from goodboy-ltd/ghx"
expect_requests "curl $pinned/v0.3.0-rc.1/ghx_linux_arm64" "curl $pinned/v0.3.0-rc.1/checksums.txt"
expect_installed "$home/.local/bin" 0.3.0-rc.1 linux_arm64
finish

begin "GHX_REPO picks another repository (darwin x86_64)"
uname_s=Darwin
install GHX_REPO=example-org/ghx-fork
expect_success
expect_output "Installing ghx latest from example-org/ghx-fork"
other="https://github.com/example-org/ghx-fork/releases/latest/download"
expect_requests "curl $other/ghx_darwin_amd64" "curl $other/checksums.txt"
expect_installed "$home/.local/bin" 0.9.0 darwin_amd64
finish

nl='
'
for bad in ../evil goodboy-ltd goodboy-ltd/ /ghx good.boy/ghx good_boy/ghx \
  goodboy-ltd/. goodboy-ltd/.. goodboy-ltd/ghx/extra 'goodboy-ltd/g hx' \
  'goodboy-ltd/ghx?x=1' 'goodboy-ltd/ghx#x' "goodboy-ltd/ghx${nl}evil" 'gööd/ghx'; do
  begin "invalid GHX_REPO fails before any download: $(printf '%s' "$bad" | tr '\n' '~')"
  install GHX_REPO="$bad"
  expect_failure
  expect_output "GHX_REPO must be owner/name"
  expect_no_requests
  expect_absent "$home/.local"
  finish
done

for bad in 1.2 1 1.2.3.4 v vv1.2.3 latest 1.2.x 1..3 .1.2 1.2. 1.2.3- \
  1.2.3-rc-1 1.2.3-rc_1 1.2.3+build 1.2.3/../x ' 1.2.3' "1.2.3${nl}4"; do
  begin "invalid GHX_VERSION fails before any download: $(printf '%s' "$bad" | tr '\n' '~')"
  install GHX_VERSION="$bad"
  expect_failure
  expect_output "GHX_VERSION must be a version"
  expect_no_requests
  expect_absent "$home/.local"
  finish
done

begin "checksum mismatch fails and leaves the existing install untouched"
mkdir -p "$home/.local/bin"
echo old > "$home/.local/bin/ghx"
install GHX_VERSION=0.0.1
expect_failure
expect_output "checksum mismatch for ghx_linux_amd64"
[ "$(cat "$home/.local/bin/ghx")" = old ] || problem "the existing ghx was replaced"
expect_absent "$home/.local/bin/gh"
finish

begin "checksum mismatch on a fresh machine installs nothing"
install GHX_VERSION=0.0.1
expect_failure
expect_output "checksum mismatch for ghx_linux_amd64"
expect_absent "$home/.local"
finish

begin "checksums.txt without the asset fails"
install GHX_VERSION=0.0.2
expect_failure
expect_output "no checksum for ghx_linux_amd64"
expect_absent "$home/.local"
finish

begin "a missing release fails"
install GHX_VERSION=9.9.9
expect_failure
expect_requests "curl $pinned/v9.9.9/ghx_linux_amd64"
expect_absent "$home/.local"
finish

begin "wget is used when curl is missing"
bins="$fake/wget"
install
expect_success
expect_requests "wget $latest/ghx_linux_amd64" "wget $latest/checksums.txt"
expect_installed "$home/.local/bin" 0.2.1 linux_amd64
finish

begin "neither curl nor wget fails"
bins="$fake/none"
install
expect_failure
expect_output "curl or wget is required"
expect_absent "$home/.local/bin/ghx"
finish

if [ -e "$work/sys-shasum/shasum" ]; then
  begin "shasum -a 256 is used when sha256sum is missing"
  sys="$work/sys-shasum"
  install
  expect_success
  expect_installed "$home/.local/bin" 0.2.1 linux_amd64
  finish
else
  echo "skip shasum fallback: this host has no shasum"
fi

begin "GHX_INSTALL_DIR and GHX_SHIM=0"
install GHX_INSTALL_DIR="$home/custom" GHX_SHIM=0
expect_success
expect_installed "$home/custom" 0.2.1 linux_amd64
expect_absent "$home/custom/gh"
expect_absent "$home/.local"
finish

begin "a real gh file in the install dir is kept"
mkdir -p "$home/.local/bin"
echo real > "$home/.local/bin/gh"
install
expect_success
expect_output "Skipped gh shim"
expect_installed "$home/.local/bin" 0.2.1 linux_amd64
[ "$(cat "$home/.local/bin/gh")" = real ] || problem "the real gh file was replaced"
finish

begin "unsupported OS fails before any download"
uname_s=FreeBSD
install
expect_failure
expect_output "unsupported OS FreeBSD"
expect_no_requests
finish

begin "unsupported CPU fails before any download"
uname_m=riscv64
install
expect_failure
expect_output "unsupported CPU riscv64"
expect_no_requests
finish

echo "$cases cases, $failed failed (install.sh run by $run_sh)"
[ "$failed" = 0 ]
