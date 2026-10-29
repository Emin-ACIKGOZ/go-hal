// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"unicode/utf8"
)

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

		var result map[string]any
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if result == nil {
			t.Fatal("expected result to not be nil")
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

		var result map[string]any
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}

		if result == nil {
			t.Fatal("expected result to not be nil")
		}
	})
}

func FuzzLink_AddLink(f *testing.F) {
	f.Add("self", "/users/1")
	f.Add("item", "/items/{id}")
	f.Add("https://example.com/rels/widget", "/widgets")
	f.Add("acme:widget", "/widgets/1")

	f.Fuzz(func(t *testing.T, rel, href string) {
		if rel == "" || href == "" {
			t.Skip()
		}
		if !utf8.ValidString(rel) || !utf8.ValidString(href) {
			t.Skip()
		}

		e := &Envelope{
			Data:     nil,
			instance: New(),
			links:    make(map[string]any),
		}
		e.AddLink(Link{Rel: rel, Href: href})

		if e.links[rel] == nil {
			t.Fatalf("link %s not added", rel)
		}
	})
}

func FuzzWrapPrecomputed(f *testing.F) {
	f.Add([]byte(`{"_links":{"self":{"href":"/x"}}}`), []byte(`{"id":1}`))
	f.Add([]byte(`{"self":{"href":"/x"}}`), []byte(`{"id":1}`))
	f.Add([]byte(`{}`), []byte(`{}`))
	f.Add([]byte(`[]`), []byte(`{"id":1}`))
	f.Add([]byte(`null`), []byte(`{"id":1}`))
	f.Add([]byte(`not json`), []byte(`{"id":1}`))
	f.Add([]byte(`{"_links":{"self":{"href":"/x"}}}`), []byte(`null`))
	f.Add([]byte(`{"_links":{"self":{"href":"/x"}}}`), []byte(`[1,2]`))
	f.Add([]byte(`{"_links":{"self":{"href":"/x","href":"/y"}}}`), []byte(`{"id":1}`))

	f.Fuzz(func(t *testing.T, linksJSON, data []byte) {
		linksDuplicateFree := findDuplicateKeys(linksJSON) == nil
		dataDuplicateFree := findDuplicateKeys(data) == nil

		inst := New()
		env := inst.WrapPrecomputed(context.Background(), json.RawMessage(data), linksJSON)
		out, err := json.Marshal(env)
		if err != nil {
			// Malformed input and non-object data are allowed to fail.
			return
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
		// Raw JSON data can carry duplicate keys; the serializer does not
		// reinterpret valid input. Duplicate-free inputs must stay clean.
		if linksDuplicateFree && dataDuplicateFree {
			if err := findDuplicateKeys(out); err != nil {
				t.Fatalf("duplicate keys in output: %v\n%s", err, out)
			}
		}
	})
}

func FuzzSplicePrecomputed(f *testing.F) {
	f.Add([]byte(`{"id":1}`), []byte(`{"_links":{"self":{"href":"/x"}}}`))
	f.Add([]byte(`{}`), []byte(`{"_links":{"self":{"href":"/x"}}}`))
	f.Add([]byte(`{"id":1}`), []byte(`{"self":{"href":"/x"}}`))
	f.Add([]byte(`null`), []byte(`{"_links":{"self":{"href":"/x"}}}`))
	f.Add([]byte(`{"id":1}`), []byte(nil))
	f.Add([]byte(`not json`), []byte(`[]`))

	f.Fuzz(func(t *testing.T, data, linksJSON []byte) {
		out := splicePrecomputed(data, linksJSON)

		dataKeys, dataObject := fuzzTopLevelObjectKeys(data)
		linkKeys, linksObject := fuzzTopLevelObjectKeys(linksJSON)
		if !dataObject || !linksObject || len(data) <= 2 || len(linksJSON) <= 2 {
			return
		}

		if !json.Valid(out) {
			t.Fatalf("invalid JSON output for object inputs %s + %s: %s", data, linksJSON, out)
		}
		if !utf8.Valid(out) {
			t.Fatalf("invalid UTF-8 output: %s", out)
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
	return keys, true
}
