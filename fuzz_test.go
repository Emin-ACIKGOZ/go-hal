// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

var (
	fuzzPositiveLinks = []byte(`{"self":{"href":"/x"}}`)
	fuzzPositiveData  = []byte(`{"id":1}`)
)

// fuzzDecodeValue decodes exactly one JSON value with number preservation.
func fuzzDecodeValue(t *testing.T, data []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var value any
	if err := dec.Decode(&value); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		t.Fatalf("unexpected trailing JSON after %s", data)
	}
	return value
}

func fuzzDecodeObject(t *testing.T, data []byte) map[string]any {
	t.Helper()
	value := fuzzDecodeValue(t, data)
	obj, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("JSON value has type %T, want object", value)
	}
	return obj
}

func FuzzMarshal_WithRegisteredGenerator(f *testing.F) {
	f.Add(1, "Ada")
	f.Add(0, "")
	f.Add(-42, "受注")

	f.Fuzz(func(t *testing.T, id int, name string) {
		if !utf8.ValidString(name) {
			t.Skip()
		}

		type User struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}

		inst := New()
		RegisterInstance(inst, func(_ context.Context, u *User) []Link {
			return []Link{{Rel: "self", Href: "/users/" + strconv.Itoa(u.ID)}}
		})

		env := inst.Wrap(context.Background(), &User{ID: id, Name: name})
		out, err := json.Marshal(env)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		if len(out) == 0 {
			t.Fatal("marshal returned empty output")
		}

		result := fuzzDecodeObject(t, out)
		if _, ok := result["_links"].(map[string]any); !ok {
			t.Fatalf("_links missing from %s", out)
		}
	})
}

func FuzzMarshal_WithoutGenerator(f *testing.F) {
	f.Add(1)
	f.Add(0)
	f.Add(-1)

	f.Fuzz(func(t *testing.T, id int) {
		type User struct {
			ID int `json:"id"`
		}

		inst := New()
		env := inst.Wrap(context.Background(), &User{ID: id})
		out, err := json.Marshal(env)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		if len(out) == 0 {
			t.Fatal("marshal returned empty output")
		}

		result := fuzzDecodeObject(t, out)
		if _, exists := result["_links"]; exists {
			t.Fatalf("unexpected _links without a generator: %s", out)
		}
	})
}

func FuzzNormalizeLink(f *testing.F) {
	f.Add("/orders{?id}", false)
	f.Add("{+path}", false)
	f.Add("/orders/%7Bid%7D", false)
	f.Add("/orders{=id}", false)
	f.Add("/orders", true)
	f.Add("", false)

	f.Fuzz(func(t *testing.T, href string, templated bool) {
		if !utf8.ValidString(href) {
			t.Skip()
		}
		in := Link{Rel: "find", Href: href, Templated: templated}
		normalized := normalizeLink(in)
		if in.Templated != templated || in.Href != href {
			t.Fatalf("normalizeLink mutated its input: %#v", in)
		}
		want := templated || deriveTemplated(href)
		if normalized.Templated != want {
			t.Fatalf("normalizeLink(%q, %t).Templated = %t, want %t",
				href, templated, normalized.Templated, want)
		}
		if twice := normalizeLink(normalized); twice != normalized {
			t.Fatalf("normalizeLink is not idempotent for %q: %#v then %#v",
				href, normalized, twice)
		}
		_ = ValidateLink(in) // must not panic on arbitrary input
	})
}

func FuzzLink_AddLink(f *testing.F) {
	f.Add("self", "/users/1")
	f.Add("item", "/items/{id}")
	f.Add("https://example.com/rels/widget", "/widgets")
	f.Add("acme:widget", "/widgets/1")

	f.Fuzz(func(t *testing.T, rel, href string) {
		if strings.TrimSpace(rel) == "" || href == "" {
			t.Skip()
		}
		if !utf8.ValidString(rel) || !utf8.ValidString(href) {
			t.Skip()
		}
		if err := validateRelationSyntax(rel); err != nil {
			t.Skip() // Default hardening rejects blank/control/whitespace relations.
		}

		e := &Envelope{
			Data:     nil,
			instance: New(),
			links:    make(map[string]any),
		}
		e.AddLink(Link{Rel: rel, Href: href})

		out, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		assertNoDuplicateKeys(t, out)
		root := fuzzDecodeObject(t, out)
		links, ok := root["_links"].(map[string]any)
		if !ok {
			t.Fatalf("_links missing from %s", out)
		}
		if _, exists := links[rel]; !exists {
			t.Fatalf("link %q missing from %s", rel, out)
		}
	})
}

func FuzzWrapPrecomputed(f *testing.F) {
	f.Add([]byte(`{"_links":{"self":{"href":"/x"}}}`), []byte(`{"id":1}`))
	f.Add([]byte(" \n {\"_links\":{\"self\":{\"href\":\"/x\"}}} \n"), []byte(`{"id":1}`))
	f.Add([]byte(`{"self":{"href":"/x"}}`), []byte(`{"id":1}`))
	f.Add([]byte(`{}`), []byte(`{}`))
	f.Add([]byte(`{ }`), []byte(`{ }`))
	f.Add([]byte(`[]`), []byte(`{"id":1}`))
	f.Add([]byte(`null`), []byte(`{"id":1}`))
	f.Add([]byte(`not json`), []byte(`{"id":1}`))
	f.Add([]byte(`{"_links":{"self":{"href":"/x"}}}`), []byte(`null`))
	f.Add([]byte(`{"_links":{"self":{"href":"/x"}}}`), []byte(`[1,2]`))
	f.Add([]byte(`{"_links":{"self":{"href":"/x","href":"/y"}}}`), []byte(`{"id":1}`))
	f.Add([]byte(`{"find":{"href":"/orders{?id}"}}`), []byte(`{"id":1}`))
	f.Add(fuzzPositiveLinks, fuzzPositiveData)

	f.Fuzz(func(t *testing.T, linksJSON, data []byte) {
		linksDuplicateFree := findDuplicateKeys(linksJSON) == nil
		dataDuplicateFree := findDuplicateKeys(data) == nil
		_, _, linksErr := normalizePrecomputedLinks(linksJSON)

		inst := New()
		env := inst.WrapPrecomputed(context.Background(), json.RawMessage(data), linksJSON)
		out, err := json.Marshal(env)
		if err != nil {
			if bytes.Equal(linksJSON, fuzzPositiveLinks) && bytes.Equal(data, fuzzPositiveData) {
				t.Fatalf("positive control rejected: %v", err)
			}
			if len(out) != 0 {
				t.Fatalf("error returned partial output: %s", out)
			}
			return
		}

		if len(out) == 0 {
			t.Fatal("successful marshal returned empty output")
		}
		if !json.Valid(out) {
			t.Fatalf("invalid JSON output: %s", out)
		}
		if !utf8.Valid(out) {
			t.Fatalf("invalid UTF-8 output: %s", out)
		}
		trimmed := bytes.TrimSpace(out)
		if len(trimmed) == 0 || trimmed[0] != '{' {
			t.Fatalf("output root is not an object: %s", out)
		}

		if bytes.Equal(linksJSON, fuzzPositiveLinks) && bytes.Equal(data, fuzzPositiveData) {
			root := fuzzDecodeObject(t, out)
			links, ok := root["_links"].(map[string]any)
			if !ok {
				t.Fatalf("positive control lost _links: %s", out)
			}
			self, ok := links["self"].(map[string]any)
			if !ok || self["href"] != "/x" {
				t.Fatalf("positive control lost self href: %s", out)
			}
		}

		dataKeys, dataObject := fuzzTopLevelObjectKeys(data)
		_, hasLinks := dataKeys[fieldLinks]
		_, hasEmbedded := dataKeys[fieldEmbedded]
		if linksErr == nil && dataObject && (hasLinks || hasEmbedded) {
			t.Fatalf("reserved field conflict was not reported: %s", out)
		}

		// Raw JSON data can carry duplicate keys; the serializer does not
		// reinterpret valid input. Duplicate-free inputs must stay clean.
		if linksDuplicateFree && dataDuplicateFree {
			if err := findDuplicateKeys(out); err != nil {
				t.Fatalf("duplicate keys in output: %v\n%s", err, out)
			}
		}
	})
}

func FuzzStrictPrecomputedGuard(f *testing.F) {
	f.Add(fuzzPositiveLinks, fuzzPositiveData)
	f.Add([]byte(`{"find":{"href":"/orders{?id}"}}`), []byte(`{"id":1}`))
	f.Add([]byte(`{"find":{"href":"/orders{?id}","templated":true}}`), []byte(`{"id":1}`))
	f.Add([]byte(`{"curies":[{"name":"acme","href":"https://example.com/{rel}"}]}`), []byte(`{"id":1}`))
	f.Add([]byte(`{"curies":[{"name":"acme","href":"https://example.com/{rel}","templated":true}]}`), []byte(`{"id":1}`))
	f.Add([]byte(`{"self":{"href":"/x"}}`), []byte(`{"a":1,"a":2}`))

	f.Fuzz(func(t *testing.T, linksJSON, data []byte) {
		inst := New(
			WithStrictLinks(),
			WithStrictRelations(),
			WithStrictCuries(),
			WithStrictJSON(),
		)
		env := inst.WrapPrecomputed(context.Background(), json.RawMessage(data), linksJSON)
		out, err := json.Marshal(env)
		if err != nil {
			if len(out) != 0 {
				t.Fatalf("error returned partial output: %s", out)
			}
			return
		}
		if len(out) == 0 {
			t.Fatal("successful marshal returned empty output")
		}
		if !json.Valid(out) {
			t.Fatalf("invalid JSON output: %s", out)
		}
		if !utf8.Valid(out) {
			t.Fatalf("invalid UTF-8 output: %s", out)
		}
	})
}

func FuzzSplicePrecomputed(f *testing.F) {
	f.Add([]byte(`{"id":1}`), []byte(`{"_links":{"self":{"href":"/x"}}}`))
	f.Add([]byte(`{}`), []byte(`{"_links":{"self":{"href":"/x"}}}`))
	f.Add([]byte(`{ }`), []byte(`{"_links":{"self":{"href":"/x"}}}`))
	f.Add([]byte(`{"id":1}`), []byte(`{"self":{"href":"/x"}}`))
	f.Add([]byte(`{"id":1}`), []byte(`{ }`))
	f.Add([]byte(`null`), []byte(`{"_links":{"self":{"href":"/x"}}}`))
	f.Add([]byte(`{"id":1}`), []byte(nil))
	f.Add([]byte(`not json`), []byte(`[]`))

	f.Fuzz(func(t *testing.T, data, linksJSON []byte) {
		if !utf8.Valid(data) || !utf8.Valid(linksJSON) {
			return
		}

		out := splicePrecomputed(data, linksJSON)

		dataKeys, dataObject := fuzzTopLevelObjectKeys(data)
		linkKeys, linksObject := fuzzTopLevelObjectKeys(linksJSON)
		if !dataObject || !linksObject {
			return
		}
		if len(out) == 0 {
			t.Fatalf("object splice produced empty output for %s + %s", data, linksJSON)
		}

		if !json.Valid(out) {
			t.Fatalf("invalid JSON output for object inputs %s + %s: %s", data, linksJSON, out)
		}
		if !utf8.Valid(out) {
			t.Fatalf("invalid UTF-8 output: %s", out)
		}
		if isEmptyJSONObject(data) {
			if isEmptyJSONObject(linksJSON) {
				if !isEmptyJSONObject(out) {
					t.Fatalf("both empty objects produced %s", out)
				}
				return
			}
			if !bytes.Equal(bytes.TrimSpace(out), bytes.TrimSpace(linksJSON)) {
				t.Fatalf("empty data lost links: %s", out)
			}
			return
		}
		if isEmptyJSONObject(linksJSON) {
			if !bytes.Equal(bytes.TrimSpace(out), bytes.TrimSpace(data)) {
				t.Fatalf("empty links lost data: %s", out)
			}
			return
		}

		overlap := false
		for key := range dataKeys {
			if _, ok := linkKeys[key]; ok {
				overlap = true
				break
			}
		}
		if !overlap {
			if err := findDuplicateKeys(out); err != nil {
				t.Fatalf("duplicate keys in output: %v\n%s", err, out)
			}
		}
	})
}

func fuzzTopLevelObjectKeys(data []byte) (map[string]struct{}, bool) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, false
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, false
	}

	keys := make(map[string]struct{})
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, false
		}
		key, ok := tok.(string)
		if !ok {
			return nil, false
		}
		keys[key] = struct{}{}
		var skip any
		if err := dec.Decode(&skip); err != nil {
			return nil, false
		}
	}

	// Consume the closing brace and reject trailing data; otherwise an input
	// like `{}0` would be mistaken for a complete object.
	if _, err := dec.Token(); err != nil {
		return nil, false
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, false
	}
	return keys, true
}
