#!/bin/sh
set -eu

: "${GITHUB_SHA:?GITHUB_SHA is required}"
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"

# The CI job serializes tag creation; refresh tags inside that lock.
git fetch --quiet origin refs/heads/main:refs/remotes/origin/main --tags
if [ "$(git rev-parse refs/remotes/origin/main)" != "$GITHUB_SHA" ]; then
    echo 'main has advanced; its newer CI run will publish the release.'
    exit 0
fi

# Ignore prereleases and unrelated tags. Git sorts numeric version components.
first_stable_tag() {
    awk '/^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/ && !found { print; found = 1 }'
}

release_tag=$(git tag --points-at "$GITHUB_SHA" --sort=-version:refname | first_stable_tag)
if [ -z "$release_tag" ]; then
    latest_tag=$(git tag --list --sort=-version:refname | first_stable_tag)
    if [ -z "$latest_tag" ]; then
        release_tag=v0.1.0
    else
        version_prefix=${latest_tag%.*}
        patch=${latest_tag##*.}
        release_tag="${version_prefix}.$((patch + 1))"
    fi

    # main may have changed while tags were being fetched and inspected.
    main_sha=$(git ls-remote --exit-code origin refs/heads/main | awk '{print $1}')
    if [ "$main_sha" != "$GITHUB_SHA" ]; then
        echo 'main has advanced; its newer CI run will publish the release.'
        exit 0
    fi
    git tag "$release_tag" "$GITHUB_SHA"
    git push origin "refs/tags/${release_tag}"
fi

# Query errors must fail the job so a retry can recover using the same tag.
published_tags=$(gh api --paginate "repos/${GITHUB_REPOSITORY}/releases" \
    --jq '.[] | select(.draft == false) | .tag_name')
if printf '%s\n' "$published_tags" | grep -Fxq "$release_tag"; then
    echo "$release_tag is already published."
    exit 0
fi

active_runs=$(gh run list --repo "$GITHUB_REPOSITORY" --workflow release.yml \
    --branch "$release_tag" --limit 100 --json status \
    --jq 'map(select(.status != "completed")) | length')
if [ "$active_runs" -gt 0 ]; then
    echo "$release_tag already has an active release run."
    exit 0
fi

# GITHUB_TOKEN tag pushes do not trigger workflows; dispatch explicitly.
gh workflow run release.yml --repo "$GITHUB_REPOSITORY" --ref "$release_tag"
echo "Started release for $release_tag."
