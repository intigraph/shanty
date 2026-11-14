# Releases

Shanty uses Release Please to propose a release PR on `main`. Conventional
`fix:` and `feat:` commits accumulate into that PR. Review its changelog and
version, wait for all CI, then merge through the normal review lane. The merged
release PR creates a `vX.Y.Z` tag and a draft release. The existing **Release**
workflow checks out that tag, runs `go test ./...`, and uses GoReleaser to build
four Linux/macOS archives plus checksums and publish the draft. Do not cut a
second hand tag when the release PR or publishing job is pending.

The initial manifest starts at the already published `v0.4.3`. The bootstrap
commit is that existing tag, so onboarding does not replay older releases. Go
binaries continue to get their version from the release tag through GoReleaser;
the manifest does not change local development version reporting.

## Built-in automation token

Release Please uses the job-scoped `GITHUB_TOKEN` with contents and pull-request
write permissions. Repository Actions must allow creating pull requests. No
additional token or secret provisioning is required.

GitHub suppresses automatic workflows from PRs and tags created with this token.
For a bot-created release PR, run the existing CI workflow explicitly on its
branch (`gh workflow run ci.yml --ref <release-pr-branch>`), then require its
complete checks and independent review through the normal lane.
When Release Please creates a release, its job outputs call the reusable Release
workflow directly with the exact tag; publication does not depend on another
tag-push event. The publisher verifies the tag and runs its existing tests before
GoReleaser. A manual Release entrypoint accepts an existing version tag for
diagnosed recovery; it does not create tags.

## Verification and recovery

Source CI does not prove activation. Retain the first actual release PR,
complete-green head, independent review, merge SHA, exact-tag Release run (direct call or tag trigger),
and asset checksums. Then confirm the ordinary release-mode census reports
`release-pr`; keep the rollout item open until both observations exist.

If a release job fails, inspect its failing step and retry only through the
approved job-recovery lane. Do not move a tag, publish untested binaries, or
create a replacement release as a workaround. To stop automation, revert the
configuration change through the normal source lane; preserve historical tags
and release assets. Disabling automation does not roll back installed binaries.
