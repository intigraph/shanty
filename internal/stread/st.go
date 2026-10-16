// Package stread is shanty's ONE reader for shantytown's `st` CLI.
//
// Why a package and not another exec.Command in each caller: the status bar, the
// session picker, and anything added later must agree about three things that are
// easy to get quietly wrong.
//
//  1. WHERE `st` IS. tmux runs `#(...)` status commands from the SERVER's
//     environment, not the pane's. A tmux server started outside a login shell
//     routinely has a narrower PATH than the user does — no ~/.local/bin — so
//     exec.LookPath("st") fails and every st-backed segment renders empty while
//     `st` works perfectly in every pane. That failure looks exactly like "you do
//     not run shantytown", which is why it can sit undetected for weeks. Bin()
//     therefore looks past PATH into the conventional install directories.
//
//  2. WHERE `st` IS RUN FROM. st resolves its tracker store by walking up from
//     the working directory. The tmux server's cwd is wherever the server was
//     started, which need not be the deployment's tracker root — and st answers a
//     lookup it cannot resolve with EMPTY and exit 0, so the bar renders blank
//     rather than failing. $SHANTY_ST_CWD names the directory to run st from.
//     It is host configuration on purpose: a tracker location must never be
//     compiled into this source.
//
//  3. HOW OFTEN. Every `#(shanty seg X)` is a separate short-lived process, so a
//     process-local cache saves nothing — each render paid full price. st costs
//     ~0.5s per call and the bar polls every few seconds across several segments,
//     which is enough to keep a core busy doing nothing. The cache here is on
//     disk and therefore shared by every segment process.
//
// Errors are returned, never swallowed. Callers decide between hiding (st is not
// installed — a machine without shantytown must see the plain bar) and rendering
// LOUD (st is installed and could not answer). A segment that cannot tell those
// apart renders a convincing blank, which is the failure mode this package exists
// to prevent.
package stread

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrNotInstalled means no `st` binary could be found anywhere we look. It is the
// one error that justifies hiding a segment instead of shouting: shanty is
// usable without shantytown, and that user must not see warnings about a tool
// they never installed.
var ErrNotInstalled = errors.New("st not installed")

// CacheTTL is how long an st answer is reused across segment processes. The bar
// polls every few seconds; plate items and crew rosters change on the order of
// minutes, so a few seconds of staleness is invisible while the saved process
// spawns are not.
const CacheTTL = 15 * time.Second

// searchDirs are the conventional install locations checked after PATH. This list
// is what makes the bar survive a tmux server whose PATH is narrower than the
// user's — see the package comment.
var searchDirs = []string{
	".local/bin", // per-user pip/uv installs (~ expanded below)
}

var absoluteSearchDirs = []string{
	"/usr/local/bin",
	"/opt/homebrew/bin",
	"/usr/bin",
}

var (
	binOnce sync.Once
	binPath string
)

// Bin returns the path to the st binary, or "" when none exists.
//
// $SHANTY_ST_BIN wins: an operator who knows where st lives should not have to
// argue with our search order. Then PATH, then the conventional directories.
func Bin() string {
	binOnce.Do(func() { binPath = findBin() })
	return binPath
}

func findBin() string {
	if p := os.Getenv("SHANTY_ST_BIN"); p != "" {
		if isExec(p) {
			return p
		}
		// An explicit override that does not resolve is a configuration error, not
		// a reason to fall back and pretend: st is then reported missing and the
		// caller renders its "not installed" branch rather than silently using a
		// different binary than the operator named.
		return ""
	}
	if p, err := exec.LookPath("st"); err == nil {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, d := range searchDirs {
			if p := filepath.Join(home, d, "st"); isExec(p) {
				return p
			}
		}
	}
	for _, d := range absoluteSearchDirs {
		if p := filepath.Join(d, "st"); isExec(p) {
			return p
		}
	}
	return ""
}

// ResetBinForTest clears the memoized binary lookup so a test can change
// $SHANTY_ST_BIN / PATH and have the next call resolve again. Resolution is
// memoized because it happens several times per render and cannot change inside
// one short-lived segment process — but a test process is long-lived.
func ResetBinForTest() {
	binOnce = sync.Once{}
	binPath = ""
}

func isExec(p string) bool {
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		return false
	}
	return fi.Mode().Perm()&0o111 != 0
}

// Installed reports whether an st binary exists at all.
func Installed() bool { return Bin() != "" }

// workDir is the directory st is run from, or "" to inherit ours. See the package
// comment: st resolves its tracker by walking up from here, so a wrong cwd yields
// an empty answer with a zero exit.
func workDir() string {
	d := os.Getenv("SHANTY_ST_CWD")
	if d == "" {
		return ""
	}
	if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
		return ""
	}
	return d
}

// Run executes st with the given arguments and returns trimmed stdout.
//
// Stdout is returned even alongside an error. st uses a nonzero exit to say
// things that are not failures — `st agent stats` exits 1 to report that no capture
// store exists yet, and prints that explanation on stdout — so a Run that threw
// the output away would force callers to render "error" where st gave them a
// perfectly clear answer.
//
// The answer is cached on disk for CacheTTL, keyed by the binary, the working
// directory, and the arguments. Failures use a separate bounded backoff entry:
// callers still receive the original output and an error, never an empty success.
// Cache hits do not extend the deadline; expiry attempts a fresh command.
func Run(args ...string) (string, error) {
	bin := Bin()
	if bin == "" {
		return "", ErrNotInstalled
	}
	dir := workDir()
	key := cacheKey(bin, dir, args)
	if raw, ok := cacheGet(key + ".failure"); ok {
		var failure cachedFailure
		if json.Unmarshal([]byte(raw), &failure) == nil && failure.Error != "" {
			return failure.Output, errors.New(failure.Error)
		}
	}
	if v, ok := cacheGet(key); ok {
		return v, nil
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = childEnv(bin)
	out, err := cmd.Output()
	val := strings.TrimSpace(string(out))
	if err != nil {
		var ee *exec.ExitError
		var reported error
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			reported = fmt.Errorf("st %s: %w: %s", strings.Join(args, " "), err, firstLine(ee.Stderr))
		} else {
			reported = fmt.Errorf("st %s: %w", strings.Join(args, " "), err)
		}
		if raw, marshalErr := json.Marshal(cachedFailure{Output: val, Error: reported.Error()}); marshalErr == nil {
			cacheSet(key+".failure", string(raw))
		}
		cacheRemove(key)
		return val, reported
	}
	cacheRemove(key + ".failure")
	cacheSet(key, val)
	return val, nil
}

// childEnv is our environment with the st binary's own directory prepended to
// PATH.
//
// Finding `st` ourselves is not sufficient, because st is not self-contained: it
// shells out to its tracker's CLI, which is installed alongside it. Under a tmux
// server whose PATH lacks that directory, `st anchor` fails with
//
//	bd list failed: [Errno 2] No such file or directory: 'bd'
//
// while `st` itself ran perfectly — a segment that is loud about a fault the
// operator cannot see the cause of. Prepending st's own directory makes st's
// companions resolvable by the same reasoning that let us find st: they were
// installed together. It PREPENDS rather than replaces, so a deliberately
// configured PATH still wins for anything else.
func childEnv(bin string) []string {
	dir := filepath.Dir(bin)
	if dir == "" || dir == "." {
		return nil // inherit ours; nothing useful to add
	}
	env := os.Environ()
	for i, kv := range env {
		if name, val, ok := strings.Cut(kv, "="); ok && name == "PATH" {
			if val == dir || strings.HasPrefix(val, dir+string(os.PathListSeparator)) {
				return env // already leading; do not grow the variable every call
			}
			env[i] = "PATH=" + dir + string(os.PathListSeparator) + val
			return env
		}
	}
	return append(env, "PATH="+dir)
}

func firstLine(b []byte) string {
	s := strings.TrimSpace(string(b))
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// --- cross-process cache ---------------------------------------------------

// cacheDisabled lets tests exercise the real command path without a cache file
// from another run answering for them.
func cacheDisabled() bool { return os.Getenv("SHANTY_SEG_NOCACHE") != "" }

func cacheKey(bin, dir string, args []string) string {
	h := sha256.Sum256([]byte(bin + "\x00" + dir + "\x00" + strings.Join(args, "\x00")))
	return hex.EncodeToString(h[:16])
}

func cacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "shanty", "seg")
}

func cacheGet(key string) (string, bool) {
	if cacheDisabled() {
		return "", false
	}
	dir := cacheDir()
	if dir == "" {
		return "", false
	}
	p := filepath.Join(dir, key)
	fi, err := os.Stat(p)
	if err != nil || time.Since(fi.ModTime()) > CacheTTL {
		return "", false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	return string(b), true
}

func cacheSet(key, val string) {
	if cacheDisabled() {
		return
	}
	dir := cacheDir()
	if dir == "" {
		return
	}
	if os.MkdirAll(dir, 0o700) != nil {
		return
	}
	// Temp-file-plus-rename: several segment processes write concurrently every
	// interval, and a half-written cache file read by another would be worse than
	// a miss. A failed write is simply a miss next time — never an error a status
	// bar has to explain.
	tmp, err := os.CreateTemp(dir, key+".*")
	if err != nil {
		return
	}
	name := tmp.Name()
	if _, err := tmp.WriteString(val); err != nil {
		tmp.Close()
		os.Remove(name)
		return
	}
	if tmp.Close() != nil {
		os.Remove(name)
		return
	}
	if os.Rename(name, filepath.Join(dir, key)) != nil {
		os.Remove(name)
	}
}

// A cached failure remains a failure. Its age is bounded by CacheTTL just like
// successful answers, and reading it never refreshes its modification time.
type cachedFailure struct {
	Output string `json:"output"`
	Error  string `json:"error"`
}

func cacheRemove(key string) {
	if cacheDisabled() {
		return
	}
	if dir := cacheDir(); dir != "" {
		_ = os.Remove(filepath.Join(dir, key))
	}
}
