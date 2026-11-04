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

## Activation prerequisite

An owner must configure the repository Actions secret `RELEASE_PLEASE_TOKEN`.
Use an automation identity with access only to this repository and the rights to
write contents and pull requests. Set an explicit expiry. Repository Actions
must allow creating pull requests. Keep the credential in Actions secrets;
never commit it or print its value. Revoke/rotate it through its owner.
From the reviewed checkout, run `bash scripts/configure-release-token.sh` and
paste once at the hidden prompt. The script reads stdin, sets only the named
repository secret, then verifies its NAME exists. It never prints the value.

The default `GITHUB_TOKEN` deliberately is **not** a fallback. GitHub suppresses
new workflow runs for PR/tag events created with that token, which would leave
release PRs without CI and tags without the binary publisher. The workflow
fails with a named prerequisite when the token is absent. After provisioning,
the owner may run **Release PR** with its normal workflow-dispatch entrypoint,
or wait for the next main push. No source change is required to activate it.
See the [upstream token contract](https://github.com/googleapis/release-please-action#other-actions-on-release-please-prs).

## Verification and recovery

Source CI does not prove activation. Retain the first actual release PR,
complete-green head, independent review, merge SHA, tag-triggered Release run,
and asset checksums. Then confirm the ordinary release-mode census reports
`release-pr`; keep the rollout item open until both observations exist.

If a release job fails, inspect its failing step and retry only through the
approved job-recovery lane. Do not move a tag, publish untested binaries, or
create a replacement release as a workaround. To stop automation, revert the
configuration change through the normal source lane; preserve historical tags
and release assets. Disabling automation does not roll back installed binaries.
