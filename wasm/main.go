//go:build js && wasm

package main

import "syscall/js"

// main installs globalThis.adfast and parks forever. Nothing below this
// line makes a decision: every export forwards straight into the bridge
// functions in api.go (which are host-testable), and every one of them
// runs inside bridgeGuard so a panic cannot cross the js.FuncOf boundary
// and tear down the instance.
//
// The js.Func values are intentionally never Released: they live for the
// lifetime of the module, and releasing them would unregister the API.
func main() {
	js.Global().Set("adfast", map[string]any{
		"scanSpans": js.FuncOf(func(_ js.Value, args []js.Value) any {
			return bridgeGuard(func() (string, error) {
				return bridgeScanSpans(stringArg(args, 0))
			})
		}),
		"toADF": js.FuncOf(func(_ js.Value, args []js.Value) any {
			return bridgeGuard(func() (string, error) {
				return bridgeToADF(stringArg(args, 0), stringArg(args, 1))
			})
		}),
		"toMarkdown": js.FuncOf(func(_ js.Value, args []js.Value) any {
			return bridgeGuard(func() (string, error) {
				return bridgeToMarkdown(stringArg(args, 0), stringArg(args, 1))
			})
		}),
		"diagnostics": js.FuncOf(func(_ js.Value, args []js.Value) any {
			return bridgeGuard(func() (string, error) {
				return bridgeDiagnostics(stringArg(args, 0), stringArg(args, 1))
			})
		}),
	})
	select {}
}

// stringArg renders argument i as the string the bridge layer expects:
// strings pass through, anything else goes through JSON.stringify (so
// toADF's `opts` and toMarkdown's `adf` may be live JS objects), and a
// missing, undefined, or null argument becomes "".
func stringArg(args []js.Value, i int) string {
	if i >= len(args) {
		return ""
	}
	switch v := args[i]; v.Type() {
	case js.TypeUndefined, js.TypeNull:
		return ""
	case js.TypeString:
		return v.String()
	default:
		return js.Global().Get("JSON").Call("stringify", v).String()
	}
}
