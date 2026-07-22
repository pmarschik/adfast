package adf

// Helpers over raw attribute maps (JSON numbers decode as float64):
// RawNode attrs, Extra maps, and nested payloads like the blockCard
// datasource shape.

// StrAttr reads a string attribute, defaulting to "".
func StrAttr(attrs map[string]any, key string) string {
	if attrs == nil {
		return ""
	}
	if v, ok := attrs[key].(string); ok {
		return v
	}
	return ""
}

// IntAttr reads an integer attribute (accepting the float64 form JSON
// decoding produces), defaulting to def.
func IntAttr(attrs map[string]any, key string, def int) int {
	if attrs == nil {
		return def
	}
	switch v := attrs[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return def
}

// BoolAttr reads a bool attribute, defaulting to false.
func BoolAttr(attrs map[string]any, key string) bool {
	v, ok := attrs[key].(bool)
	return ok && v
}
