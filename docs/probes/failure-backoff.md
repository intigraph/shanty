# Failed status reads keep their error during backoff

Three successive failed status renders previously launched three `st` commands.
The same injected command now launches once within the existing 15-second cache
window. Every caller receives its partial output and the same error; the bar
continues to report that it cannot read the status.

Run the regression without a running tracker:

```sh
go test ./internal/stread -run '^TestFailureBackoffRemainsUnknownAndExpires$' -v
```

The fixture uses a temporary executable that returns partial output, an error
and exit 2. It checks the invocation count, preserved output/error, a deadline
that does not extend on hits, isolation by arguments, expiry, recovery and the
existing successful-result cache. Expiry is simulated by aging only the fixture
cache file; the test does not sleep or contact a service.

Recorded handler probe: `three failed renders: subprocess calls=3` before,
`subprocess calls=1` after, with UNKNOWN preserved in both cases. This is a
command-count demonstration, not a production request-rate measurement. The
initial concurrent cache misses can still launch multiple commands; this change
does not serialize readers or claim to measure their fleet-wide contribution.
