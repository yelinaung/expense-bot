#!/usr/bin/env bash
set -euo pipefail

branch="$(git rev-parse --abbrev-ref HEAD)"
if [[ "${branch}" == "HEAD" ]]; then
	echo "Detached HEAD; please checkout a branch before pushing."
	exit 1
fi

remotes="$(git remote)"
if [[ -z "${remotes}" ]]; then
	echo "No git remotes configured."
	exit 1
fi

for remote in ${remotes}; do
	echo "Pushing ${branch} to ${remote}..."
	git push "${remote}" "${branch}"
done
