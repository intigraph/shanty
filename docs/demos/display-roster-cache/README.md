# Display roster reuse

A status render sees a 60-second-old successful crew observation. The display
reader reuses it; the interactive reader refreshes it. When the observation is
older than five minutes and the producer is unavailable, the display fails
rather than presenting stale success.

The recording calls both library readers on the same synthetic fixture. The
interactive `Crew` reader is unchanged from base 6c061877eb19bafa2a3723da8e471b2f91f9743a;
`DisplayCrew` is the candidate reader. This is source behavior, not an installed
request-rate measurement. Code is in `internal/stread/display_test.go`.

Repeat: `go test -v ./internal/stread -run TestDisplayRosterReuseDoesNotExtendInteractiveFreshness -count=1`.
Replay the retained [output](output.log) and [timing](timing.log):
`scriptreplay --log-out output.log --log-timing timing.log`.
Playback verified with util-linux scriptreplay. These small sanitized artifacts
are retained in Git alongside the reproducer.
