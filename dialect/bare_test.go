package dialect

import (
	"testing"

	"github.com/pmarschik/adfast/ast"
	"github.com/pmarschik/adfast/extension"
)

// extension.Registration.Validate builds its prototype from a BLANK
// directive, which is exactly the bare shape, so every guarded
// constructor hands it a *bareDirective and the structural check no
// longer reaches the real kinds. This runs the same check against the
// kinds themselves, on a directive that carries a payload.
//
// The requirements are extension.validatePrototype's, restated here
// because that method is unexported: a constructor returns a non-nil
// node, the node is an ast.Parent so tree walks reach its children, and
// an inline node is an extension.InlineLead so neighbor escape checks
// see its first byte.
func TestTextPrototypes(t *testing.T) {
	for _, reg := range unguardedRegistrations() {
		for name, ctor := range reg.Texts {
			t.Run(reg.Kind+"/"+name, func(t *testing.T) {
				n := ctor(&ast.TextDirective{
					Name:     name,
					Attrs:    map[string]string{"probe": "1"},
					Children: []ast.Node{&ast.Text{Value: "x"}},
				})
				if n == nil {
					t.Fatal("constructor returned nil")
				}
				if _, ok := ast.Node(n).(ast.Parent); !ok {
					t.Errorf("%T does not implement ast.Parent", n)
				}
				if _, ok := ast.Node(n).(extension.InlineLead); !ok {
					t.Errorf("%T does not implement extension.InlineLead", n)
				}
			})
		}
	}
}

// The guard is a property of the assembled set, not of any one kind:
// every text name the dialect registers goes through it, so a name added
// later is guarded by construction. This pins that, comparing the raw
// set against the shipped one.
func TestEveryTextNameIsGuarded(t *testing.T) {
	guarded := map[string]bool{}
	for _, reg := range Registrations() {
		for name, ctor := range reg.Texts {
			guarded[name] = true
			if _, ok := ctor(&ast.TextDirective{Name: name}).(*bareDirective); !ok {
				t.Errorf("%s: a bare %q is not prose", reg.Kind, name)
			}
			// A payload still reaches the real constructor.
			if _, ok := ctor(&ast.TextDirective{
				Name:  name,
				Attrs: map[string]string{"probe": "1"},
			}).(*bareDirective); ok {
				t.Errorf("%s: an attributed %q was taken for prose", reg.Kind, name)
			}
		}
	}
	for _, reg := range unguardedRegistrations() {
		for name := range reg.Texts {
			if !guarded[name] {
				t.Errorf("%s: text name %q is not in the shipped set", reg.Kind, name)
			}
		}
	}
	if len(guarded) == 0 {
		t.Fatal("no text names registered; the guard covers nothing")
	}
}

// The bare node reports the name as the text it is spelled as, so the
// generic walks (ast.PlainText, and the image alt built from it) read
// the word rather than nothing.
func TestBareDirectiveCarriesItsText(t *testing.T) {
	n := newBareDirective("media")
	if got := ast.PlainText([]ast.Node{n}); got != ":media" {
		t.Errorf("PlainText = %q, want %q", got, ":media")
	}
}

// unguardedRegistrations assembles the same set Registrations() does,
// without the guard in front of it.
func unguardedRegistrations() []extension.Registration {
	regs := blockRegistrations()
	regs = append(regs, inlineRegistrations()...)
	regs = append(regs, markRegistrations()...)
	regs = append(regs, extendedRegistrations()...)
	return regs
}
