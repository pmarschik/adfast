package convert

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Codes() is only worth anything if it is COMPLETE: a consumer classifying
// every diagnostic code proves exhaustiveness against it, so an inventory
// missing a code turns that consumer's green test into a false one. The
// const block is the source of truth, and this file re-derives the
// inventory from the source rather than from a second hand-written list —
// a hand-written expectation would drift exactly the way the inventory can.

// codeConstantFloor is a sanity floor for the source scan. Its job is the
// VACUOUS case: a rename of the constants, a moved file or a broken parse
// would otherwise leave the comparison below with two empty sets, which
// agree perfectly and prove nothing. The number is deliberately well under
// the real count so that adding a code does not have to touch it.
const codeConstantFloor = 20

// codeConstant is one exported Code* constant as the source declares it.
type codeConstant struct {
	Name        string // the identifier, e.g. "CodeJQLDegraded"
	Value       string // the code string, e.g. "jql-degraded"
	ViaSelector bool   // declared as another package's constant (adf.CodeX)
}

// parseCodeConstants returns every exported Code* constant declared in the
// non-test Go source of dir, in declaration order, with its string value
// resolved. Selector forms (the adf re-exports) resolve through imported,
// which maps the other package's constant NAMES to their values.
//
// A declaration form it does not understand is a FATAL error rather than a
// skip: a code the scan cannot read is a code the comparison below would
// silently stop covering, which is the failure this whole file exists to
// prevent.
func parseCodeConstants(t *testing.T, dir string, imported map[string]string) []codeConstant {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatalf("globbing %s: %v", dir, err)
	}
	slices.Sort(files) // declaration order is only stable if file order is

	var out []codeConstant
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		out = append(out, codeConstantsInFile(t, file, imported)...)
	}
	return out
}

// codeConstantsInFile is parseCodeConstants for one file.
func codeConstantsInFile(t *testing.T, file string, imported map[string]string) []codeConstant {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parsing %s: %v", file, err)
	}

	var out []codeConstant
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			out = append(out, codeConstantsInSpec(t, file, value, imported)...)
		}
	}
	return out
}

// codeConstantsInSpec reads the exported Code* names out of one const
// spec, resolving each one's value.
func codeConstantsInSpec(t *testing.T, file string, spec *ast.ValueSpec, imported map[string]string) []codeConstant {
	t.Helper()

	var out []codeConstant
	for i, name := range spec.Names {
		if !strings.HasPrefix(name.Name, "Code") || !name.IsExported() {
			continue
		}
		if i >= len(spec.Values) {
			t.Fatalf("%s: constant %s has no value expression; the scan cannot resolve it",
				file, name.Name)
		}
		code, viaSelector := resolveCodeValue(t, file, name.Name, spec.Values[i], imported)
		out = append(out, codeConstant{Name: name.Name, Value: code, ViaSelector: viaSelector})
	}
	return out
}

// resolveCodeValue reads one constant's value out of its declaration: a
// string literal, or another package's constant resolved through imported.
func resolveCodeValue(t *testing.T, file, name string, expr ast.Expr, imported map[string]string) (string, bool) {
	t.Helper()

	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			t.Fatalf("%s: constant %s is a %s literal, not a string", file, name, e.Kind)
		}
		code, err := strconv.Unquote(e.Value)
		if err != nil {
			t.Fatalf("%s: unquoting %s = %s: %v", file, name, e.Value, err)
		}
		return code, false
	case *ast.SelectorExpr:
		pkg, ok := e.X.(*ast.Ident)
		if !ok || pkg.Name != "adf" {
			t.Fatalf("%s: constant %s is declared from an unexpected package selector; "+
				"teach parseCodeConstants to resolve it", file, name)
		}
		code, ok := imported[e.Sel.Name]
		if !ok {
			t.Fatalf("%s: constant %s = adf.%s, which the adf scan did not find",
				file, name, e.Sel.Name)
		}
		return code, true
	default:
		t.Fatalf("%s: constant %s is declared as %T, a form parseCodeConstants cannot resolve; "+
			"teach it that form, because a code it cannot read is a code the inventory test stops covering",
			file, name, expr)
		return "", false
	}
}

// declaredCodeConstants scans adf first (so the re-exports resolve) and
// then this package, and applies the vacuity floor.
func declaredCodeConstants(t *testing.T) []codeConstant {
	t.Helper()

	adfValues := map[string]string{}
	for _, c := range parseCodeConstants(t, "../adf", nil) {
		adfValues[c.Name] = c.Value
	}
	if len(adfValues) == 0 {
		t.Fatal("no Code* constants found in ../adf; the scan is broken, not the inventory")
	}

	declared := parseCodeConstants(t, ".", adfValues)
	if len(declared) < codeConstantFloor {
		t.Fatalf("found only %d Code* constants in package convert (floor %d); "+
			"the source scan is broken, so the comparison below would prove nothing",
			len(declared), codeConstantFloor)
	}
	if !slices.ContainsFunc(declared, func(c codeConstant) bool { return c.ViaSelector }) {
		t.Error("no constant resolved through the adf selector form; " +
			"resolveCodeValue's re-export branch is no longer exercised")
	}
	return declared
}

func TestCodesMatchesEveryDeclaredConstant(t *testing.T) {
	declared := declaredCodeConstants(t)

	want := make([]string, 0, len(declared))
	byValue := map[string]string{}
	for _, c := range declared {
		want = append(want, c.Value)
		if prior, dup := byValue[c.Value]; dup {
			t.Errorf("constants %s and %s both declare the code %q", prior, c.Name, c.Value)
		}
		byValue[c.Value] = c.Name
	}

	got := Codes()
	if slices.Equal(got, want) {
		return
	}

	// Report what is wrong by NAME, so the failure names the constant to
	// add rather than only the string that is absent.
	for _, c := range declared {
		if !slices.Contains(got, c.Value) {
			t.Errorf("%s (%q) is declared but missing from the Codes() inventory", c.Name, c.Value)
		}
	}
	seen := map[string]int{}
	for _, code := range got {
		seen[code]++
		if _, ok := byValue[code]; !ok {
			t.Errorf("Codes() reports %q, which no exported Code* constant declares", code)
		} else if seen[code] == 2 {
			t.Errorf("Codes() reports %q (%s) more than once", code, byValue[code])
		}
	}
	if len(got) == len(want) {
		t.Errorf("Codes() holds the right set in the wrong order; the inventory must follow "+
			"the declaration order of the constants\n got: %v\nwant: %v", got, want)
	}
}

// TestCodesReturnsAFreshSlice pins the copy: the inventory is package
// state, and a consumer that sorts the result must not reorder it for
// everyone else.
func TestCodesReturnsAFreshSlice(t *testing.T) {
	first := Codes()
	if len(first) < codeConstantFloor {
		t.Fatalf("Codes() returned %d codes, below the floor %d", len(first), codeConstantFloor)
	}
	original := slices.Clone(first)

	for i := range first {
		first[i] = fmt.Sprintf("clobbered-%d", i)
	}
	if second := Codes(); !slices.Equal(second, original) {
		t.Errorf("mutating the returned slice changed the inventory:\n got: %v\nwant: %v", second, original)
	}
}
