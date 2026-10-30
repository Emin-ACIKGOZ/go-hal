// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDuplicateJSONKey_TypedError(t *testing.T) {
	t.Parallel()

	err := findDuplicateJSONKeys([]byte(`{"a":1,"a":2}`))
	if !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("error = %v, want ErrDuplicateJSONKey", err)
	}
	var dup *DuplicateKeyError
	if !errors.As(err, &dup) {
		t.Fatalf("error %v is not *DuplicateKeyError", err)
	}
	if dup.Key != "a" {
		t.Fatalf("Key = %q, want a", dup.Key)
	}
	if dup.Offset <= 0 {
		t.Fatalf("Offset = %d, want the duplicate key position", dup.Offset)
	}
	if !strings.Contains(err.Error(), `duplicate JSON key "a"`) {
		t.Fatalf("Error() = %q", err)
	}
}

func TestDuplicateJSONKey_Nested(t *testing.T) {
	t.Parallel()

	err := findDuplicateJSONKeys([]byte(`{"outer":{"x":1,"x":2}}`))
	var dup *DuplicateKeyError
	if !errors.As(err, &dup) {
		t.Fatalf("error %v is not *DuplicateKeyError", err)
	}
	if dup.Key != "x" {
		t.Fatalf("Key = %q, want x", dup.Key)
	}
}

func TestDuplicateJSONKey_CapsDisplayedKey(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", 200)
	err := findDuplicateJSONKeys([]byte(`{"` + long + `":1,"` + long + `":2}`))
	var dup *DuplicateKeyError
	if !errors.As(err, &dup) {
		t.Fatalf("error %v is not *DuplicateKeyError", err)
	}
	if len(dup.Key) > duplicateKeyDisplayLimit+3 {
		t.Fatalf("Key length = %d, want capped display", len(dup.Key))
	}
	if !strings.HasSuffix(dup.Key, "...") {
		t.Fatalf("capped Key = %q, want ellipsis", dup.Key)
	}
}

func TestDuplicateJSONKey_ThroughPrecomputed(t *testing.T) {
	t.Parallel()

	links := []byte(`{"self":{"href":"/x","href":"/y"}}`)
	_, err := CompilePrecomputedLinks(links)
	if !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("compile error = %v, want ErrDuplicateJSONKey", err)
	}
	var dup *DuplicateKeyError
	if !errors.As(err, &dup) || dup.Key != "href" {
		t.Fatalf("compile error does not expose DuplicateKeyError: %v", err)
	}

	env := New().WrapPrecomputed(context.Background(), &embeddedTestUser{ID: 1}, links)
	_, err = json.Marshal(env)
	if !errors.Is(err, ErrDuplicateJSONKey) {
		t.Fatalf("marshal error = %v, want ErrDuplicateJSONKey", err)
	}
	if !errors.As(err, &dup) {
		t.Fatalf("marshal error does not expose DuplicateKeyError: %v", err)
	}
}

func BenchmarkFindDuplicateJSONKeys(b *testing.B) {
	data := []byte(`{"_links":{"self":{"href":"/users/1"},"item":[{"href":"/items/1"},{"href":"/items/2"}]},"id":1,"name":"Alice"}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := findDuplicateJSONKeys(data); err != nil {
			b.Fatal(err)
		}
	}
}

func TestDuplicateJSONKey_OffsetPointsAtDuplicateKey(t *testing.T) {
	t.Parallel()

	data := []byte(`{"a":1, "a":2}`)
	err := findDuplicateJSONKeys(data)
	var dup *DuplicateKeyError
	if !errors.As(err, &dup) {
		t.Fatalf("error %v is not *DuplicateKeyError", err)
	}
	want := strings.LastIndex(string(data), `"a"`)
	if int(dup.Offset) != want {
		t.Fatalf("Offset = %d, want duplicate key start %d", dup.Offset, want)
	}
	if dup.Offset >= int64(len(data)) {
		t.Fatalf("Offset = %d is outside input length %d", dup.Offset, len(data))
	}
}
