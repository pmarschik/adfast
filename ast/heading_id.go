package ast

import "regexp"

// HeadingIDPattern is the character grammar of Heading.ID — the id in a
// "## Title {#id}" heading anchor. It opens with an ASCII alphanumeric and
// continues in alphanumerics, '-', '_' and '.': pandoc's auto-id charset,
// and exactly the set that survives as plain heading text.
//
// The restriction is what makes the markdown surface reversible. A ':'
// opens a text directive and a '*', '`' or '[' opens an inline span, so an
// id containing one would not reach the renderer as plain text; a space
// would end the anchor. Ids outside the grammar therefore cannot be
// written as a "{#id}" suffix at all, which is why a product addon lifting
// its own anchor construct into Heading.ID must check first (see
// confluence.LiftAnchors) and fall back to a form that does round trip.
//
// The pattern is unanchored so it can be embedded; use ValidHeadingID to
// test a whole id.
const HeadingIDPattern = `[0-9A-Za-z][0-9A-Za-z._-]*`

var headingIDRe = regexp.MustCompile(`^` + HeadingIDPattern + `$`)

// ValidHeadingID reports whether id can be written as a "{#id}" heading
// anchor — i.e. whether it matches HeadingIDPattern in full. An empty id
// is not valid ("{#}" is literal text, not an anchor).
func ValidHeadingID(id string) bool { return headingIDRe.MatchString(id) }
