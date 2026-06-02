package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"reflect"
	"regexp"

	"github.com/flowbaker/flowbaker/pkg/expressions"
)

type Expect struct {
	Value   json.RawMessage `json:"value,omitempty"`
	Error   bool            `json:"error,omitempty"`
	Matcher string          `json:"matcher,omitempty"`
	Pattern string          `json:"pattern,omitempty"`
}

type Vector struct {
	ID       string         `json:"id"`
	Expr     string         `json:"expr,omitempty"`
	Template string         `json:"template,omitempty"`
	Scope    map[string]any `json:"scope"`
	Expect   Expect         `json:"expect"`
}

type Suite struct {
	Version int      `json:"version"`
	Vectors []Vector `json:"vectors"`
}

func main() {
	path := flag.String("vectors", "pkg/expressions/vectors.json", "path to vectors.json")
	failFast := flag.Bool("fail-fast", false, "stop at first mismatch")
	flag.Parse()

	raw, err := os.ReadFile(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read vectors:", err)
		os.Exit(2)
	}
	var suite Suite
	if err := json.Unmarshal(raw, &suite); err != nil {
		fmt.Fprintln(os.Stderr, "parse vectors:", err)
		os.Exit(2)
	}

	binder, err := expressions.NewCELBinder(expressions.DefaultCELBinderOptions())
	if err != nil {
		fmt.Fprintln(os.Stderr, "init binder:", err)
		os.Exit(2)
	}

	ctx := context.Background()
	pass, fail := 0, 0
	for _, v := range suite.Vectors {
		text := v.Template
		if text == "" {
			text = "{{ " + v.Expr + " }}"
		}
		item := v.Scope["item"]
		got, runErr := binder.BindString(ctx, item, text)

		ok, why := check(v, got, runErr)
		if ok {
			pass++
			continue
		}
		fail++
		fmt.Printf("FAIL %s\n  expr/template: %s\n  scope: %s\n  %s\n",
			v.ID, displayInput(v), mustJSON(v.Scope), why)
		if *failFast {
			break
		}
	}

	fmt.Printf("\n%d passed, %d failed, %d total\n", pass, fail, pass+fail)
	if fail > 0 {
		os.Exit(1)
	}
}

func check(v Vector, got any, runErr error) (bool, string) {
	if v.Expect.Error {
		if runErr == nil {
			return false, fmt.Sprintf("expected error, got value: %s", mustJSON(got))
		}
		return true, ""
	}
	if runErr != nil {
		return false, "unexpected error: " + runErr.Error()
	}
	if v.Expect.Matcher == "regex" {
		s, ok := got.(string)
		if !ok {
			return false, fmt.Sprintf("regex matcher needs string, got %T: %v", got, got)
		}
		re, err := regexp.Compile(v.Expect.Pattern)
		if err != nil {
			return false, "bad regex pattern: " + err.Error()
		}
		if !re.MatchString(s) {
			return false, fmt.Sprintf("regex %q did not match %q", v.Expect.Pattern, s)
		}
		return true, ""
	}

	var want any
	if len(v.Expect.Value) > 0 {
		if err := json.Unmarshal(v.Expect.Value, &want); err != nil {
			return false, "bad expect.value JSON: " + err.Error()
		}
	}
	gotN := normalizeForCompare(got)
	wantN := normalizeForCompare(want)
	if reflect.DeepEqual(gotN, wantN) {
		return true, ""
	}
	return false, fmt.Sprintf("got %s, want %s", mustJSON(gotN), mustJSON(wantN))
}

func normalizeForCompare(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return v
	}
	return out
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func displayInput(v Vector) string {
	if v.Template != "" {
		return "(template) " + v.Template
	}
	return "(expr) " + v.Expr
}
