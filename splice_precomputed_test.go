// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"testing"
)

func TestSplicePrecomputed_EmptyData(t *testing.T) {
	links := []byte(`{"_links":{"self":{}}}`)

	result := splicePrecomputed([]byte("{}"), links)
	if string(result) != string(links) {
		t.Fatalf("expected links for empty data: %s", result)
	}
}

func TestSplicePrecomputed_EmptyLinks(t *testing.T) {
	data := []byte(`{"id":1}`)

	result := splicePrecomputed(data, nil)
	if string(result) != string(data) {
		t.Fatalf("expected data for empty links: %s", result)
	}
}

func TestSplicePrecomputed_BothPresent(t *testing.T) {
	data := []byte(`{"id":1}`)
	links := []byte(`{"_links":{"self":{}}}`)

	result := splicePrecomputed(data, links)
	assertJSONEqual(t, result, []byte(`{"id":1,"_links":{"self":{}}}`))
}

func TestSplicePrecomputed_WhitespaceObjects(t *testing.T) {
	links := []byte(`{"_links":{"self":{"href":"/x"}}}`)

	for _, data := range []string{`{ }`, "{\n}", "{\t \r\n}"} {
		t.Run("empty data "+data, func(t *testing.T) {
			result := splicePrecomputed([]byte(data), links)
			assertJSONEqual(t, result, links)
		})
	}

	for _, emptyLinks := range []string{`{ }`, "{\n}", "{\t \r\n}"} {
		t.Run("empty links "+emptyLinks, func(t *testing.T) {
			result := splicePrecomputed([]byte(`{"id":1}`), []byte(emptyLinks))
			assertJSONEqual(t, result, []byte(`{"id":1}`))
		})
	}
}

func TestSplicePrecomputed_NullAndEmpty(t *testing.T) {
	links := []byte(`{"_links":{"self":{"href":"/x"}}}`)
	assertJSONEqual(t, splicePrecomputed([]byte("null"), links), links)
	assertJSONEqual(t, splicePrecomputed(nil, links), links)
	assertJSONEqual(t, splicePrecomputed([]byte(`{"id":1}`), nil), []byte(`{"id":1}`))
}

func TestCheckJSONStructure_WhitespaceObject(t *testing.T) {
	for _, data := range []string{`{ }`, "{\n}", "{\t\r\n}"} {
		isNull, isEmpty, err := checkJSONStructure([]byte(data))
		if err != nil {
			t.Fatalf("%q: %v", data, err)
		}
		if isNull || !isEmpty {
			t.Fatalf("%q: isNull=%t isEmpty=%t, want empty object", data, isNull, isEmpty)
		}
	}
}
