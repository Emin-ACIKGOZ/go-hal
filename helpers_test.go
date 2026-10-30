// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// decodeAny decodes exactly one JSON value, keeping numbers as json.Number and
// rejecting any trailing JSON.
func decodeAny(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("decode JSON %s: %w", data, err)
	}
	if err := checkNoTrailing(dec, data); err != nil {
		return nil, err
	}
	return v, nil
}

// decodeJSONValue is decodeAny with a fail-closed testing wrapper.
func decodeJSONValue(t *testing.T, data []byte) any {
	t.Helper()
	v, err := decodeAny(data)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// decodeJSONObject decodes exactly one JSON object, keeping numbers as
// json.Number and rejecting any trailing JSON.
func decodeJSONObject(t *testing.T, data []byte) map[string]json.RawMessage {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil {
		t.Fatalf("decode JSON object %s: %v", data, err)
	}
	if err := checkNoTrailing(dec, data); err != nil {
		t.Fatal(err)
	}
	return obj
}

func checkNoTrailing(dec *json.Decoder, data []byte) error {
	var trailing any
	err := dec.Decode(&trailing)
	switch {
	case errors.Is(err, io.EOF):
		return nil
	case err == nil:
		return fmt.Errorf("unexpected trailing JSON value after %s", data)
	default:
		return fmt.Errorf("invalid trailing data after %s: %w", data, err)
	}
}

// jsonValuesEqual compares two JSON documents semantically, ignoring object key
// order and insignificant whitespace.
func jsonValuesEqual(got, want []byte) error {
	gotValue, err := decodeAny(got)
	if err != nil {
		return fmt.Errorf("got: %w", err)
	}
	wantValue, err := decodeAny(want)
	if err != nil {
		return fmt.Errorf("want: %w", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		return fmt.Errorf("JSON values differ\n got: %s\nwant: %s", got, want)
	}
	return nil
}

func assertJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()
	if err := jsonValuesEqual(got, want); err != nil {
		t.Fatal(err)
	}
}

func checkRequiredKeys(obj map[string]json.RawMessage, keys ...string) error {
	var missing []string
	for _, key := range keys {
		if _, ok := obj[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing JSON keys %v (present: %v)", missing, objectKeys(obj))
	}
	return nil
}

func checkForbiddenKeys(obj map[string]json.RawMessage, keys ...string) error {
	var unexpected []string
	for _, key := range keys {
		if _, ok := obj[key]; ok {
			unexpected = append(unexpected, key)
		}
	}
	if len(unexpected) > 0 {
		return fmt.Errorf("unexpected JSON keys %v (present: %v)", unexpected, objectKeys(obj))
	}
	return nil
}

func checkExactKeys(obj map[string]json.RawMessage, keys ...string) error {
	got := objectKeys(obj)
	want := append([]string(nil), keys...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("JSON object keys differ\n got: %v\nwant: %v", got, want)
	}
	return nil
}

func objectKeys(obj map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(obj))
	for key := range obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func requireKeys(t *testing.T, obj map[string]json.RawMessage, keys ...string) {
	t.Helper()
	if err := checkRequiredKeys(obj, keys...); err != nil {
		t.Fatal(err)
	}
}

func forbidKeys(t *testing.T, obj map[string]json.RawMessage, keys ...string) {
	t.Helper()
	if err := checkForbiddenKeys(obj, keys...); err != nil {
		t.Fatal(err)
	}
}

func requireExactKeys(t *testing.T, obj map[string]json.RawMessage, keys ...string) {
	t.Helper()
	if err := checkExactKeys(obj, keys...); err != nil {
		t.Fatal(err)
	}
}

// findDuplicateKeys scans data for duplicate object member names at any depth.
// Duplicate names are non-interoperable JSON and cannot be detected by decoding
// into a map.
func findDuplicateKeys(data []byte) error {
	return findDuplicateJSONKeys(data)
}

func assertNoDuplicateKeys(t *testing.T, data []byte) {
	t.Helper()
	if err := findDuplicateKeys(data); err != nil {
		t.Fatalf("invalid JSON keys in %s: %v", data, err)
	}
}

// collectRelationObjects returns the link objects for rel. HAL allows a
// relation value to be either one Link Object or an array of Link Objects.
func collectRelationObjects(root map[string]any, rel string) ([]map[string]any, error) {
	rawLinks, ok := root["_links"]
	if !ok {
		return nil, errors.New(`missing "_links" object`)
	}
	links, ok := rawLinks.(map[string]any)
	if !ok {
		return nil, fmt.Errorf(`"_links" has type %T, want object`, rawLinks)
	}

	raw, ok := links[rel]
	if !ok {
		return nil, fmt.Errorf("missing link relation %q", rel)
	}

	switch v := raw.(type) {
	case map[string]any:
		return []map[string]any{v}, nil
	case []any:
		objects := make([]map[string]any, 0, len(v))
		for i, item := range v {
			obj, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("relation %q element %d has type %T, want object", rel, i, item)
			}
			objects = append(objects, obj)
		}
		return objects, nil
	default:
		return nil, fmt.Errorf("relation %q has type %T, want object or array of objects", rel, raw)
	}
}

func relationObjects(t *testing.T, root map[string]any, rel string) []map[string]any {
	t.Helper()
	objects, err := collectRelationObjects(root, rel)
	if err != nil {
		t.Fatal(err)
	}
	return objects
}

func checkLinkHref(link map[string]any) error {
	href, ok := link["href"]
	if !ok {
		return errors.New(`missing required "href" field`)
	}
	if _, ok := href.(string); !ok {
		return fmt.Errorf(`"href" has type %T, want string`, href)
	}
	return nil
}

// recoverPanic calls fn and reports any panic as a message instead of
// propagating it.
func recoverPanic(fn func()) (message string, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			message = fmt.Sprint(r)
			panicked = true
		}
	}()
	fn()
	return "", false
}

func assertPanics(t *testing.T, want string, fn func()) {
	t.Helper()
	message, panicked := recoverPanic(fn)
	if !panicked {
		t.Fatalf("expected panic containing %q, got none", want)
	}
	if !strings.Contains(message, want) {
		t.Fatalf("panic %q does not contain %q", message, want)
	}
}

// assertRejectsNonStrictPrecomputedHandle checks the four precomputed handle
// variants shared by the strict CURIE and strict link tests.
func assertRejectsNonStrictPrecomputedHandle(t *testing.T, option InstanceOption, want error) {
	t.Helper()

	structural, err := CompilePrecomputedLinks([]byte(`{"self":{"href":"/x"}}`))
	if err != nil {
		t.Fatal(err)
	}
	strict, err := CompilePrecomputedLinksStrict([]byte(`{"self":{"href":"/x"}}`))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		handle *PrecomputedLinks
		want   error
	}{
		{name: "structural", handle: structural, want: want},
		{name: "strict", handle: strict},
		{name: "zero", handle: &PrecomputedLinks{}},
		{name: "nil", handle: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := New(option).WrapPrecomputedLinks(&embeddedTestUser{ID: 1}, tc.handle)
			b, err := json.Marshal(env)
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("error = %v (output %s), want %v", err, b, tc.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("valid handle rejected: %v", err)
			}
		})
	}
}

func TestJSONAssertionHelpers(t *testing.T) {
	t.Run("matching values pass", func(t *testing.T) {
		if err := jsonValuesEqual(
			[]byte(`{"b":2,"a":1}`),
			[]byte(`{"a":1,"b":2}`),
		); err != nil {
			t.Fatalf("equivalent JSON reported as different: %v", err)
		}
	})

	t.Run("wrong value fails", func(t *testing.T) {
		if err := jsonValuesEqual([]byte(`{"a":1}`), []byte(`{"a":2}`)); err == nil {
			t.Fatal("different values were reported as equal")
		}
	})

	t.Run("missing key fails", func(t *testing.T) {
		obj := map[string]json.RawMessage{"a": json.RawMessage(`1`)}
		if err := checkRequiredKeys(obj, "a", "b"); err == nil {
			t.Fatal("missing key was not detected")
		}
	})

	t.Run("extra key fails", func(t *testing.T) {
		obj := map[string]json.RawMessage{
			"a": json.RawMessage(`1`),
			"b": json.RawMessage(`2`),
		}
		if err := checkExactKeys(obj, "a"); err == nil {
			t.Fatal("extra key was not detected")
		}
	})

	t.Run("forbidden key fails", func(t *testing.T) {
		obj := map[string]json.RawMessage{"_embedded": json.RawMessage(`{}`)}
		if err := checkForbiddenKeys(obj, "_embedded"); err == nil {
			t.Fatal("forbidden key was not detected")
		}
	})

	t.Run("trailing JSON fails", func(t *testing.T) {
		if _, err := decodeAny([]byte(`{"a":1} {"b":2}`)); err == nil {
			t.Fatal("trailing JSON was not detected")
		}
	})
}

func TestHALContractHelpers(t *testing.T) {
	t.Run("duplicate scanner accepts clean JSON", func(t *testing.T) {
		if err := findDuplicateKeys([]byte(`{"a":[{"b":1},{"c":2}],"d":{"e":3}}`)); err != nil {
			t.Fatalf("clean JSON rejected: %v", err)
		}
	})

	t.Run("duplicate scanner rejects top-level duplicate", func(t *testing.T) {
		if err := findDuplicateKeys([]byte(`{"a":1,"a":2}`)); err == nil {
			t.Fatal("duplicate top-level key was not detected")
		}
	})

	t.Run("duplicate scanner rejects nested duplicate", func(t *testing.T) {
		if err := findDuplicateKeys([]byte(`{"outer":{"a":1,"a":2}}`)); err == nil {
			t.Fatal("duplicate nested key was not detected")
		}
	})

	t.Run("duplicate scanner rejects duplicate in array element", func(t *testing.T) {
		if err := findDuplicateKeys([]byte(`[{"a":1,"a":2}]`)); err == nil {
			t.Fatal("duplicate key inside array element was not detected")
		}
	})

	t.Run("duplicate scanner rejects malformed JSON", func(t *testing.T) {
		if err := findDuplicateKeys([]byte(`{"a":1`)); err == nil {
			t.Fatal("malformed JSON was not detected")
		}
	})

	t.Run("relation helper accepts one object", func(t *testing.T) {
		root := map[string]any{
			"_links": map[string]any{
				"self": map[string]any{"href": "/users/1"},
			},
		}
		links, err := collectRelationObjects(root, "self")
		if err != nil {
			t.Fatal(err)
		}
		if len(links) != 1 {
			t.Fatalf("got %d links, want 1", len(links))
		}
		if err := checkLinkHref(links[0]); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("relation helper accepts array of objects", func(t *testing.T) {
		root := map[string]any{
			"_links": map[string]any{
				"item": []any{
					map[string]any{"href": "/items/1"},
					map[string]any{"href": "/items/2"},
				},
			},
		}
		links, err := collectRelationObjects(root, "item")
		if err != nil {
			t.Fatal(err)
		}
		if len(links) != 2 {
			t.Fatalf("got %d links, want 2", len(links))
		}
	})

	t.Run("relation helper rejects missing links", func(t *testing.T) {
		if _, err := collectRelationObjects(map[string]any{}, "self"); err == nil {
			t.Fatal("missing _links was not detected")
		}
	})

	t.Run("relation helper rejects non-object links", func(t *testing.T) {
		root := map[string]any{"_links": []any{}}
		if _, err := collectRelationObjects(root, "self"); err == nil {
			t.Fatal("non-object _links was not detected")
		}
	})

	t.Run("relation helper rejects non-object relation", func(t *testing.T) {
		root := map[string]any{
			"_links": map[string]any{"self": "/users/1"},
		}
		if _, err := collectRelationObjects(root, "self"); err == nil {
			t.Fatal("string relation value was not detected")
		}
	})

	t.Run("relation helper rejects non-object array element", func(t *testing.T) {
		root := map[string]any{
			"_links": map[string]any{"item": []any{"/items/1"}},
		}
		if _, err := collectRelationObjects(root, "item"); err == nil {
			t.Fatal("non-object array element was not detected")
		}
	})

	t.Run("href helper rejects missing href", func(t *testing.T) {
		if err := checkLinkHref(map[string]any{"title": "x"}); err == nil {
			t.Fatal("missing href was not detected")
		}
	})

	t.Run("href helper rejects non-string href", func(t *testing.T) {
		if err := checkLinkHref(map[string]any{"href": 42}); err == nil {
			t.Fatal("non-string href was not detected")
		}
	})

	t.Run("panic helper detects panic", func(t *testing.T) {
		message, panicked := recoverPanic(func() { panic("boom") })
		if !panicked || message != "boom" {
			t.Fatalf("got (%q, %t), want (\"boom\", true)", message, panicked)
		}
	})

	t.Run("panic helper detects absence", func(t *testing.T) {
		if _, panicked := recoverPanic(func() {}); panicked {
			t.Fatal("panic reported for clean function")
		}
	})
}
