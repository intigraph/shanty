# Inbox display cache demonstration

Twelve real `shanty seg inbox` invocations use an isolated synthetic `st` executable,
HOME and cache. Cache modification times advance five seconds between calls to
model t=0,5,...55 without waiting or contacting a tracker.

| Answer | Installed v0.4.2 calls | Candidate calls |
| --- | ---: | ---: |
| Seven unread | 4 | 1 |
| Zero unread | 4 | 1 |
| Failed read | 12 | 1 |

All failed renders still display `inbox?`; zero remains hidden and seven remains
visible. Candidate implementation source is d7e1fe7775db7bd6c68849c75a964f193881adb1;
the binary was built from the unchanged working tree before that commit. The
installed v0.4.2 binary carries modified VCS metadata: its exact source binding
is unknown. This is a command-count fixture, not measured fleet query savings.

The [output transcript](inbox-cache.output) and [timing file](inbox-cache.timing)
are a real terminal recording, not a video or tested inline player. Download both
and replay with `scriptreplay --log-out inbox-cache.output --log-timing inbox-cache.timing`.
The recorded command used the retained fixture copy; repeat the same probe with:

```sh
python3 scripts/demo-inbox-cache.py --before /path/to/old/shanty --after /path/to/new/shanty
```

The process concurrency, expiry, routing, corruption and error controls are in
`go test ./internal/stread -run TestInbox -v`.
