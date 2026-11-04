# Persistent segments and protected role

Trigger: config.toml selects left=session,crewid and right=task,cpu,clock,
without stats. Base ignores the file; candidate uses it on every regeneration.
A separate same-binary probe repeats generation twice and verifies the user file
is unchanged and stats stays absent (also pinned by TestGenerateConfigPreservesUserSegmentsAcrossRegeneration).

The [terminal recording](output.log), [timing](timing.log), and
[rendered rows](render170.json) show an actual 170-column private tmux client.
The base loses ROLE:lead under an oversized right side; candidate keeps it
first on the left. Segment job expansions are replaced with deterministic
text so this tests tmux layout/clipping without querying any live crew or
changing the operator's tmux. This is a Linux private fixture, not a Mac
adoption or relaunch claim.

Base: 4549b4b920330804c5202a8fe50f0b5476cfe293. Captured candidate source SHA256:

- internal/config/status.go: `fd06bcd17f75b50c3c3799aa1568d8f4c3e4d8c21f930cb75e5ace0941a3c7e9`
- internal/config/status_file.go: `e0fd9f0732d915debb12f47e9416cfcf2964c02ea5f157e50a88ad324e75456e`
- internal/session/tmux.go: `6fe1482813451b748b5c34666663aa3e923da2955f01c96d5e35b1954a97f603`

To repeat, build base/candidate binaries separately, set SHANTY_DEMO_BASELINE
and SHANTY_DEMO_CANDIDATE to their paths, then:

```bash
uv run --no-project --with pyte python docs/demos/status-layout/run.py
scriptreplay --log-timing docs/demos/status-layout/timing.log --log-out docs/demos/status-layout/output.log
```

Playback was checked with util-linux scriptreplay. The small output-only
recording is retained in this directory for review; it is also a readable
transcript. No production board calls, daemon installation, or Mac writes
are part of this demonstration.
