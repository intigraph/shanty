package stread

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// InboxCacheTTL bounds status-bar staleness, including UNKNOWN backoff. Unlike
// the interactive inbox, the display may reuse an answer for one minute.
const InboxCacheTTL = time.Minute

// RunInboxCount is the display-only reader. Its cache is shared across short
// lived tmux processes, with one nonblocking refresh per routing scope. Neither
// cached failures nor a competing refresher can masquerade as an empty inbox.
func RunInboxCount(agent string) (string, error) {
	bin := Bin()
	if bin == "" {
		return "", ErrNotInstalled
	}
	dir := workDir()
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", errors.New("inbox working directory unavailable")
		}
	}
	args := []string{"inbox", "--count", agent}
	if cacheDisabled() {
		return inboxCommand(bin, dir, args)
	}
	key, err := inboxKey(bin, dir, args)
	if err != nil {
		return "", err
	}
	base := cacheDir()
	if base == "" {
		return "", errors.New("inbox cache unavailable")
	}
	if err := os.MkdirAll(base, 0700); err != nil {
		return "", errors.New("inbox cache unavailable")
	}
	path := filepath.Join(base, key+".inbox")
	if value, ok := inboxCached(path); ok {
		return value.Output, inboxError(value.Error)
	}
	release, err := inboxLock(path + ".lock")
	if err != nil {
		return "", errors.New("inbox refresh unavailable or already in progress")
	}
	defer release()
	// Another process may have refreshed between our first read and lock.
	if value, ok := inboxCached(path); ok {
		return value.Output, inboxError(value.Error)
	}
	out, runErr := inboxCommand(bin, dir, args)
	value := cachedFailure{Output: out}
	if runErr != nil {
		value.Error = runErr.Error()
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", errors.New("inbox cache encoding failed")
	}
	tmp, err := os.CreateTemp(base, key+".inbox-*")
	if err != nil {
		return "", errors.New("inbox cache write failed")
	}
	name := tmp.Name()
	defer os.Remove(name)
	_, writeErr := tmp.Write(raw)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil || os.Rename(name, path) != nil {
		return "", errors.New("inbox cache write failed")
	}
	return out, runErr
}

func inboxError(message string) error {
	if message == "" {
		return nil
	}
	return errors.New(message)
}

func inboxCommand(bin, dir string, args []string) (string, error) {
	out, err := runCommand(bin, dir, args)
	if err != nil {
		// This display needs only the failure state, not potentially unbounded
		// command output or stderr. Keep the backoff entry bounded and private.
		return "", errors.New("inbox count unavailable")
	}
	if !inboxCount(out) {
		return "", errors.New("inbox count is not a nonnegative integer")
	}
	return out, nil
}

func inboxCount(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}

func inboxCached(path string) (cachedFailure, bool) {
	var value cachedFailure
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > 64*1024 {
		return value, false
	}
	age := time.Since(fi.ModTime())
	if age < 0 || age >= InboxCacheTTL {
		return value, false
	}
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &value) != nil {
		return value, false
	}
	if value.Error == "" && !inboxCount(value.Output) {
		return value, false
	}
	return value, true
}

func inboxKey(bin, dir string, args []string) (string, error) {
	fi, err := os.Stat(bin)
	if err != nil {
		return "", errors.New("inbox executable unavailable")
	}
	scope := []string{bin, dir, fmt.Sprint(fi.Size()), fmt.Sprint(fi.ModTime().UnixNano())}
	scope = append(scope, args...)
	// These are route declarations, never credential contents. Hashing keeps
	// private route values out of cache filenames and diagnostics.
	for _, name := range []string{"SHANTY_SD_STORE", "SHANTY_SD_QUIPU", "SHANTY_SD_PREFIX", "QUIPU_SERVER", "SHANTY_ROOT", "SHANTY_ONTO_NS"} {
		scope = append(scope, name, os.Getenv(name))
	}
	raw, _ := json.Marshal(scope)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
