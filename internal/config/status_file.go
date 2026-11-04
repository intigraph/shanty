package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// LoadStatusBar loads persistent user choices, keeping defaults for omitted
// sides. An explicit empty array hides that side. Invalid input must fail before
// the generated tmux file is opened: a typo must not restore unwanted jobs.
func LoadStatusBar(path string, knownSegments []string) (StatusBarConfig, error) {
	cfg := DefaultStatusBar()
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return cfg, nil
	} else if err != nil {
		return StatusBarConfig{}, fmt.Errorf("reading status configuration: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return StatusBarConfig{}, fmt.Errorf("reading status configuration: %w", err)
	}
	var file struct {
		Status StatusBarConfig `toml:"status"`
	}
	meta, err := toml.Decode(string(data), &file)
	if err != nil {
		return StatusBarConfig{}, fmt.Errorf("invalid status configuration TOML")
	}
	if len(meta.Undecoded()) != 0 {
		return StatusBarConfig{}, fmt.Errorf("unknown status configuration key")
	}
	if meta.IsDefined("status", "left") {
		cfg.Left = file.Status.Left
	}
	if meta.IsDefined("status", "right") {
		cfg.Right = file.Status.Right
	}
	known := make(map[string]bool, len(knownSegments))
	for _, name := range knownSegments {
		known[name] = true
	}
	seen := map[string]bool{}
	for _, side := range [][]string{cfg.Left, cfg.Right} {
		for _, name := range side {
			if !known[name] {
				return StatusBarConfig{}, fmt.Errorf("unknown status segment %q", name)
			}
			if seen[name] && name != "crewid" {
				return StatusBarConfig{}, fmt.Errorf("duplicate status segment %q", name)
			}
			seen[name] = true
		}
	}
	return cfg, nil
}
