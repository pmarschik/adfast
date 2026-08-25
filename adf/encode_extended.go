package adf

// The encode half for the extended kinds (see nodes_extended.go and
// marks_extended.go): typed fields recombine with Extra exactly like
// encode.go's switches.

// extensionAttrs fills the shared extension-family attributes.
func extensionAttrs(a attrs, extensionType, extensionKey string, parameters any, text, layout, localID string) {
	a.str("extensionType", extensionType)
	a.str("extensionKey", extensionKey)
	a.rawAny("parameters", parameters)
	a.str("text", text)
	a.str("layout", layout)
	a.str("localId", localID)
}
