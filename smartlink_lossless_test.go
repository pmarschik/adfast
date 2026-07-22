package adfast

import (
	"regexp"
	"testing"

	"github.com/pmarschik/adfast/convert"
)

var testBareKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)

// reversibleSmartLinks shortens browse URLs to their key AND expands the
// key back under a fixed base — a round-trippable resolver.
var reversibleSmartLinks = convert.SmartLinks{
	KeyFromURL: jiraTestSmartLinks.KeyFromURL,
	URLForKey: func(key string) (string, bool) {
		if !testBareKeyRe.MatchString(key) {
			return "", false
		}
		return "https://apiary.atlassian.net/browse/" + key, true
	},
}

// TestNormalize_CardLabelShorteningIsReversibleOnly pins that the format
// pass never canonicalizes a smart-link card to a short label it cannot
// rebuild: with a shorten-only resolver the full URL is kept, and the
// format is ADF-preserving; with a reversible resolver it shortens and
// still round-trips.
func TestNormalize_CardLabelShorteningIsReversibleOnly(t *testing.T) {
	const url = "https://apiary.atlassian.net/browse/BEE-42"
	md := "::linkCard[" + url + "]\n"

	t.Run("shorten-only config keeps the url", func(t *testing.T) {
		got := fmtMD(md, WithSmartLinks(jiraTestSmartLinks))
		if got != md {
			t.Errorf("lossy shortening: got %q, want the full-url card %q", got, md)
		}
		// The canonicalization is ADF-preserving: formatting cannot change
		// what the document encodes to.
		before := marshalDoc(t, mdToADF(md, WithSmartLinks(jiraTestSmartLinks)))
		after := marshalDoc(t, mdToADF(got, WithSmartLinks(jiraTestSmartLinks)))
		if before != after {
			t.Errorf("format changed the ADF:\n before %s\n after  %s", before, after)
		}
	})

	t.Run("reversible config shortens and round-trips", func(t *testing.T) {
		got := fmtMD(md, WithSmartLinks(reversibleSmartLinks))
		if want := "::linkCard[BEE-42]\n"; got != want {
			t.Errorf("reversible shortening: got %q, want %q", got, want)
		}
		before := marshalDoc(t, mdToADF(md, WithSmartLinks(reversibleSmartLinks)))
		after := marshalDoc(t, mdToADF(got, WithSmartLinks(reversibleSmartLinks)))
		if before != after {
			t.Errorf("format changed the ADF:\n before %s\n after  %s", before, after)
		}
	})
}
