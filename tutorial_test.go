package adfast_test

import (
	"os"
	"strings"
	"testing"

	adfast "github.com/pmarschik/adfast"
)

// TestReadmeTutorialRoundTrips extracts the "complete example" document
// from the README and asserts it converts and round-trips stably, so the
// tutorial cannot rot.
func TestReadmeTutorialRoundTrips(t *testing.T) {
	raw, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	_, rest, ok := strings.Cut(s, "<!-- tutorial:begin -->")
	if !ok {
		t.Fatal("tutorial:begin marker missing")
	}
	block, _, ok := strings.Cut(rest, "<!-- tutorial:end -->")
	if !ok {
		t.Fatal("tutorial:end marker missing")
	}
	block = strings.TrimSpace(block)
	// The tutorial is exactly ONE four-backtick fenced block (four so the
	// sample itself can contain three-backtick code fences). Strip the
	// actual fences and fail loudly on any mismatch — a stray fence line
	// must not silently truncate what this test round-trips.
	const openFence = "````markdown"
	const closeFence = "````"
	if !strings.HasPrefix(block, openFence+"\n") {
		t.Fatalf("tutorial block must open with %q, got %q", openFence, firstLine(block))
	}
	block = strings.TrimPrefix(block, openFence)
	if !strings.HasSuffix(block, "\n"+closeFence) {
		t.Fatalf("tutorial block must close with %q, got %q", closeFence, lastLine(block))
	}
	block = strings.TrimSuffix(block, closeFence)
	block = strings.TrimSpace(block)
	// No backtick-fence residue may remain at the block edges: an
	// unbalanced fence inside the tutorial would surface here as a
	// leading or trailing fence line.
	for _, edge := range []string{firstLine(block), lastLine(block)} {
		if strings.HasPrefix(strings.TrimSpace(edge), "```") {
			t.Fatalf("backtick fence residue at tutorial block edge: %q (unbalanced fence inside the tutorial?)", edge)
		}
	}

	roundTrip := func(md string) string {
		return adfast.ToMarkdown(adfast.FromADF(adfast.ToADF(adfast.FromMarkdown(md))))
	}
	first := roundTrip(block)
	second := roundTrip(first)
	if first != second {
		t.Errorf("tutorial does not round-trip stably:\nfirst:  %q\nsecond: %q", first, second)
	}
	// The interesting constructs must survive the ADF trip.
	for _, want := range []string{
		":mention[Maya Winters]", ":status[In Progress]", ":date[2026-04-12]",
		":::info", ":::expand[Why a vertical hive stand?]", "::colwidths[120,80,220]",
		"::jql[project = BEE", "::linkCard[", ":::section", ":::column{width=\"50\"}",
		":::center", "![Mite counts by week](https://static.example.org/mite-counts.png \"Varroa counts, spring 2026\")",
		"::media[hive-inspection-sheet.pdf]", ":::breakout{wide}", "::decisions\n\n- we requeen Hive B",
		":annotation[inline comments]",
		":placeholder[your name here…]", "- [ ] paint the new stands",
		":::warning", ":::expand[Storage map]", ":emoji{",
		"::linkEmbed[https://wiki.example.org/apiary/map]",
		`key="scale"`, `key="weather-widget"`,
		`key="inspection-log"`, `key="season-tabs"`,
		":::frame", ":::syncBlock{", "::syncBlock{",
		":::indent{2}", ":::end", ":::dataConsumer{", ":::fragment{",
		":::media[hive stand sketch]", ":media{#7c1e0d2a-4b3f-45e8-9a2b-6c5d4e3f2a1b",
	} {
		if !strings.Contains(first, want) {
			t.Errorf("construct lost in round trip: %s", want)
		}
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

func lastLine(s string) string {
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		return s[i+1:]
	}
	return s
}
