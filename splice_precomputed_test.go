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
