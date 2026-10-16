#!/usr/bin/env bash
# Paste once, or pipe the token on stdin. Never put it in command arguments.
set +x
set -euo pipefail
release_token=''
trap 'unset release_token' EXIT
if [[ -t 0 ]]; then
  printf 'Paste Shanty automation PAT (hidden): ' >&2
fi
IFS= read -r -s release_token || [[ -n "$release_token" ]]
if [[ -t 0 ]]; then printf '\n' >&2; fi
if [[ -z "$release_token" ]]; then
  echo 'Empty token refused; no write.' >&2
  exit 2
fi
printf '%s' "$release_token" | gh secret set RELEASE_PLEASE_TOKEN --repo scbrown/shanty
unset release_token
if ! gh secret list --repo scbrown/shanty --json name --jq '.[].name' | grep -Fxq RELEASE_PLEASE_TOKEN; then
  echo 'Secret name readback failed; do not paste again without checking.' >&2
  exit 1
fi
echo 'RELEASE_PLEASE_TOKEN name verified in scbrown/shanty.'
