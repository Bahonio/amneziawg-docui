#!/bin/sh
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
test_root=$(mktemp -d "${TMPDIR:-/tmp}/awg-docui-auto-release.XXXXXX")
trap 'rm -rf "$test_root"' EXIT HUP INT TERM
mkdir -p "$test_root/bin"

# Git operations use real disposable repositories. Only GitHub is simulated.
cat > "$test_root/bin/gh" <<'EOF'
#!/bin/sh
set -eu
case "$1 ${2:-}" in
    'api --paginate')
        [ ! -f "$GH_MOCK_STATE/fail-api" ] || exit 1
        cat "$GH_MOCK_STATE/published"
        ;;
    'run list') cat "$GH_MOCK_STATE/active" ;;
    'workflow run')
        [ ! -f "$GH_MOCK_STATE/fail-dispatch" ] || exit 1
        printf '%s\n' "$*" >> "$GH_MOCK_STATE/dispatched"
        ;;
    *) echo "Unexpected gh command: $*" >&2; exit 1 ;;
esac
EOF
chmod +x "$test_root/bin/gh"
PATH="$test_root/bin:$PATH"
export PATH
GITHUB_REPOSITORY=test/automation
export GITHUB_REPOSITORY

fail() {
    echo "FAIL: $*" >&2
    cat "$fixture/output" >&2
    exit 1
}

new_fixture() {
    fixture="$test_root/$1"
    mkdir -p "$fixture/state"
    GH_MOCK_STATE="$fixture/state"
    export GH_MOCK_STATE
    : > "$GH_MOCK_STATE/published"
    : > "$GH_MOCK_STATE/dispatched"
    printf '0\n' > "$GH_MOCK_STATE/active"
    git -c init.defaultBranch=main init --quiet --bare "$fixture/remote.git"
    git -c init.defaultBranch=main init --quiet "$fixture/work"
    cd "$fixture/work"
    git config user.name 'Release Test'
    git config user.email 'release-test@example.invalid'
    git remote add origin "$fixture/remote.git"
    git commit --quiet --allow-empty -m initial
    git push --quiet origin main
    GITHUB_SHA=$(git rev-parse HEAD)
    export GITHUB_SHA
}

next_commit() {
    git commit --quiet --allow-empty -m update
    git push --quiet origin main --tags
    GITHUB_SHA=$(git rev-parse HEAD)
    export GITHUB_SHA
}

run_release() {
    sh "$script_dir/auto-release.sh" > "$fixture/output" 2>&1
}

assert_dispatch() {
    expected="workflow run release.yml --repo $GITHUB_REPOSITORY --ref $1"
    [ "$(cat "$GH_MOCK_STATE/dispatched")" = "$expected" ] || fail "expected one dispatch for $1"
    tag_sha=$(git --git-dir="$fixture/remote.git" rev-parse "refs/tags/$1^{commit}")
    [ "$tag_sha" = "$GITHUB_SHA" ] || fail 'remote tag does not match the tested commit'
}

assert_no_dispatch() {
    [ ! -s "$GH_MOCK_STATE/dispatched" ] || fail 'unexpected release dispatch'
}

new_fixture first-release
run_release
assert_dispatch v0.1.0
echo 'PASS: a repository without version tags starts at v0.1.0'

new_fixture patch-version
git tag v0.1.9
git tag v0.1.10
git tag v9.0.0-rc.1
git tag unrelated
next_commit
run_release
assert_dispatch v0.1.11
echo 'PASS: patch versions sort numerically and ignore prerelease and unrelated tags'

new_fixture minor-version
git tag v0.1.99
git tag v0.2.0
next_commit
run_release
assert_dispatch v0.2.1
echo 'PASS: automatic patches follow the newest minor version'

new_fixture annotated-tag
git tag -a v1.0.0 -m release
git push --quiet origin --tags
run_release
assert_dispatch v1.0.0
[ "$(git tag --list)" = v1.0.0 ] || fail 'retry created another tag'
echo 'PASS: an existing annotated version tag is reused'

new_fixture published-release
git tag v0.1.3
git push --quiet origin --tags
printf 'v0.1.3\n' > "$GH_MOCK_STATE/published"
run_release
assert_no_dispatch
[ "$(git tag --list)" = v0.1.3 ] || fail 'published release created another tag'
echo 'PASS: a published release is skipped on rerun'

new_fixture active-release
git tag v0.1.2
next_commit
printf '1\n' > "$GH_MOCK_STATE/active"
run_release
assert_no_dispatch
printf '0\n' > "$GH_MOCK_STATE/active"
run_release
assert_dispatch v0.1.3
[ "$(git tag --list | wc -l | tr -d ' ')" = 2 ] || fail 'retry created another version'
echo 'PASS: an active release is skipped and a failed release retries with the same tag'

new_fixture stale-commit
stale_sha=$GITHUB_SHA
next_commit
git checkout --quiet --detach "$stale_sha"
GITHUB_SHA=$stale_sha
export GITHUB_SHA
run_release
assert_no_dispatch
[ -z "$(git --git-dir="$fixture/remote.git" tag --list)" ] || fail 'stale CI created a tag'
echo 'PASS: CI for a superseded main commit cannot create a release'

new_fixture api-failure
touch "$GH_MOCK_STATE/fail-api"
if run_release; then fail 'GitHub API failure was ignored'; fi
assert_no_dispatch
rm "$GH_MOCK_STATE/fail-api"
run_release
assert_dispatch v0.1.0
[ "$(git tag --list)" = v0.1.0 ] || fail 'API failure retry created another version'
echo 'PASS: GitHub API errors fail the job and a retry recovers its existing tag'

new_fixture dispatch-failure
touch "$GH_MOCK_STATE/fail-dispatch"
if run_release; then fail 'workflow dispatch failure was ignored'; fi
assert_no_dispatch
rm "$GH_MOCK_STATE/fail-dispatch"
run_release
assert_dispatch v0.1.0
[ "$(git tag --list)" = v0.1.0 ] || fail 'dispatch failure retry created another version'
echo 'PASS: workflow dispatch errors can be retried without increasing the version'
