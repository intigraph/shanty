package segments

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/intigraph/shanty/internal/stread"
	"runtime"
)

func TestClockRender(t *testing.T) {
	c := Clock{}
	if c.Name() != "clock" {
		t.Errorf("expected name 'clock', got %q", c.Name())
	}
	result := c.Render()
	// Should contain HH:MM format wrapped in Dracula cyan tmux color
	if !strings.Contains(result, "#[fg=#8be9fd]") {
		t.Errorf("expected Dracula cyan color code, got %q", result)
	}
	if !strings.Contains(result, ":") {
		t.Errorf("expected time with colon, got %q", result)
	}
}

func TestHostRender(t *testing.T) {
	h := Host{}
	if h.Name() != "host" {
		t.Errorf("expected name 'host', got %q", h.Name())
	}
	result := h.Render()
	if result == "" {
		t.Error("expected non-empty hostname")
	}
	// Should be wrapped in Dracula green tmux color
	if !strings.Contains(result, "#[fg=#50fa7b]") {
		t.Errorf("expected Dracula green color code, got %q", result)
	}
}

func TestLoadRender(t *testing.T) {
	l := Load{}
	if l.Name() != "load" {
		t.Errorf("expected name 'load', got %q", l.Name())
	}
	result := l.Render()
	if !strings.HasPrefix(result, "load ") {
		t.Errorf("expected an identified load value, got %q", result)
	}
}

func TestMemRender(t *testing.T) {
	m := Mem{}
	if m.Name() != "mem" {
		t.Errorf("expected name 'mem', got %q", m.Name())
	}
	result := m.Render()
	// Should contain a percentage or "n/a"
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !strings.Contains(result, "mem ") {
		t.Errorf("expected a memory label in result, got %q", result)
	}
	if result != "mem n/a" && !strings.Contains(result, "%") {
		t.Errorf("expected percentage in result, got %q", result)
	}
}

func TestDiskRender(t *testing.T) {
	d := Disk{}
	if d.Name() != "disk" {
		t.Errorf("expected name 'disk', got %q", d.Name())
	}
	result := d.Render()
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !strings.Contains(result, "disk ") {
		t.Errorf("expected a disk label in result, got %q", result)
	}
	if result != "disk n/a" && !strings.Contains(result, "%") {
		t.Errorf("expected percentage in result, got %q", result)
	}
}

func TestCPURenderCarriesIdentity(t *testing.T) {
	result := CPU{}.Render()
	if !strings.Contains(result, "cpu ") {
		t.Errorf("expected a CPU label in result, got %q", result)
	}
}

func TestColorForPercent(t *testing.T) {
	tests := []struct {
		pct      float64
		expected string
	}{
		{0, "#50fa7b"},   // green
		{49, "#50fa7b"},  // green
		{50, "#ffb86c"},  // orange
		{79, "#ffb86c"},  // orange
		{80, "#ff5555"},  // red
		{100, "#ff5555"}, // red
	}
	for _, tt := range tests {
		got := colorForPercent(tt.pct)
		if got != tt.expected {
			t.Errorf("colorForPercent(%.0f) = %q, want %q", tt.pct, got, tt.expected)
		}
	}
}

func TestRegistryContainsAllSegments(t *testing.T) {
	expected := []string{"session", "clock", "host", "cpu", "mem", "load", "disk",
		"crewid", "task", "stats", "anchor", "crew", "events", "inbox", "harness"}
	for _, name := range expected {
		if _, ok := Registry[name]; !ok {
			t.Errorf("Registry missing segment %q", name)
		}
	}
}

func TestAllNamesMatchesRegistry(t *testing.T) {
	names := AllNames()
	if len(names) != len(Registry) {
		t.Errorf("AllNames() has %d entries, Registry has %d", len(names), len(Registry))
	}
	for _, name := range names {
		if _, ok := Registry[name]; !ok {
			t.Errorf("AllNames() contains %q but it's not in Registry", name)
		}
	}
}

func shantytownSegments() []Segment {
	return []Segment{CrewID{}, Task{}, Stats{}, Anchor{}, Crew{}, Events{}, Inbox{}, Harness{}}
}

func TestShantytownSegmentsEmptyWithoutST(t *testing.T) {
	// Without an st binary anywhere, shantytown segments render empty. This is
	// the ONE silent-empty the segments allow: shanty works without shantytown,
	// and that user must not be warned about a tool they never installed.
	//
	// SHANTY_ST_BIN pointing at nothing is how we force that state — Bin() treats
	// an override it cannot resolve as "no st" rather than searching on, so this
	// holds even on a host where st is installed.
	t.Setenv("SHANTY_ST_BIN", filepath.Join(t.TempDir(), "definitely-not-st"))
	stread.ResetBinForTest()
	defer stread.ResetBinForTest()

	for _, seg := range shantytownSegments() {
		if got := seg.Render(); got != "" {
			t.Errorf("segment %q returned %q with no st installed, want empty", seg.Name(), got)
		}
	}
}

func TestPerAgentSegmentsAreLoudWithoutIdentity(t *testing.T) {
	// The contract this asserts REPLACED an earlier one that rendered empty when
	// the identity could not be resolved. Empty was the bug: a bar segment that
	// blanks looks like "nothing to report" and gets believed, so an entire fleet
	// bar sat blank and read as healthy. With st present and no identity
	// derivable, every per-agent segment must say something.
	if !stread.Installed() {
		t.Skip("no st on this host — the loud branch requires st to be installed")
	}
	t.Setenv("SHANTY_AGENT", "")
	SetSession("") // no prefix to derive from either
	defer SetSession("")

	for _, seg := range shantytownSegments() {
		if seg.Name() == "crew" {
			continue // crew is fleet-wide; it needs no identity
		}
		got := seg.Render()
		if got == "" {
			t.Errorf("segment %q rendered empty with no identity — must be loud", seg.Name())
		}
		if !strings.Contains(got, "⚠") {
			t.Errorf("segment %q rendered %q with no identity, want a ⚠ marker", seg.Name(), got)
		}
	}
}

func TestClipKeepsRunesWhole(t *testing.T) {
	// A byte-slice through a multi-byte character would put a broken glyph in the
	// status line, so the clip counts runes.
	long := "réécrire le cache très très très très long"
	got := clip(long, 12)
	if []rune(got)[len([]rune(got))-1] != '…' {
		t.Errorf("clip(%q) = %q, want a trailing ellipsis", long, got)
	}
	if n := len([]rune(got)); n != 12 {
		t.Errorf("clip produced %d runes, want 12: %q", n, got)
	}
	if short := clip("short", 12); short != "short" {
		t.Errorf("clip left a short string alone? got %q", short)
	}
}

func TestHuman(t *testing.T) {
	for n, want := range map[int]string{
		0: "0", 950: "950", 1000: "1k", 1500: "1.5k",
		12_000: "12k", 1_000_000: "1M", 3_450_000: "3.5M",
	} {
		if got := human(n); got != want {
			t.Errorf("human(%d) = %q, want %q", n, got, want)
		}
	}
}

// TestCPUAndMemReportRealNumbersOnThisPlatform is the regression test for
// Stiwi's 2026-09-21 report: "cpu and memory segments are not reported correctly
// and show as n/a".
//
// It asserts on THIS machine, whichever that is, because the defect was exactly a
// platform assumption: both segments read /proc, so on a BSD userland they
// rendered a permanent "n/a" while the machine was perfectly capable of
// answering. A test that mocked the reader would have passed throughout.
//
// "n/a" is still a legal answer for a platform neither reader understands — this
// only refuses it on a platform we claim to support, which is any platform this
// suite is being run on.
func TestCPUAndMemReportRealNumbersOnThisPlatform(t *testing.T) {
	for _, tc := range []struct {
		name string
		seg  Segment
	}{{"cpu", CPU{}}, {"mem", Mem{}}} {
		got := tc.seg.Render()
		if strings.Contains(got, "n/a") {
			t.Errorf("%s segment renders %q on %s/%s — it reads a source this "+
				"platform does not have", tc.name, got, runtime.GOOS, runtime.GOARCH)
		}
		if !strings.Contains(got, "%") {
			t.Errorf("%s segment renders %q, which carries no percentage",
				tc.name, got)
		}
	}
}

// TestMemUsesTheCompressorFootprintNotItsContents pins the one number in this
// area that is easy to get wrong by a factor of three.
//
// macOS reports both "Pages stored in compressor" (the UNCOMPRESSED size of
// everything the compressor holds) and "Pages occupied by compressor" (what it
// actually costs in RAM). On the machine this was written on the first was 20GiB
// against 16GiB of physical memory — a number that is not merely wrong but
// impossible, and which would have pinned the segment red forever.
func TestMemUsesTheCompressorFootprintNotItsContents(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("reads vm_stat, which is a macOS tool")
	}
	pct, ok := memPercent()
	if !ok {
		t.Fatal("memPercent could not read this machine")
	}
	if pct <= 0 || pct > 100 {
		t.Errorf("memory is %v%%, which is not a share of anything", pct)
	}
}
