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
