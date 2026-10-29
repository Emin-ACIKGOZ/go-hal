// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	goccyjson "github.com/goccy/go-json"
)

var errCustomMarshal = errors.New("custom marshal failed")

type customMarshaler struct {
	payload string
	err     error
}

func (m customMarshaler) MarshalJSON() ([]byte, error) {
	if m.err != nil {
		return nil, m.err
	}
	return []byte(m.payload), nil
}

func TestEncoderMatrix_CustomMarshalers(t *testing.T) {
	encoders := []struct {
		name    string
		marshal func(any) ([]byte, error)
	}{
		{name: "encoding/json", marshal: json.Marshal},
		{name: "goccy/go-json", marshal: goccyjson.Marshal},
	}

	links := `{"_links":{"self":{"href":"/x"}}}`
	tests := []struct {
		name      string
		data      customMarshaler
		want      []byte
		wantErr   bool
		wantErrIs error
	}{
		{
			name: "object",
			data: customMarshaler{payload: `{"id":1}`},
			want: []byte(`{"id":1,"_links":{"self":{"href":"/x"}}}`),
		},
		{
			name: "whitespace padded object",
			data: customMarshaler{payload: " \n {\"id\":1} \n"},
			want: []byte(`{"id":1,"_links":{"self":{"href":"/x"}}}`),
		},
		{
			name: "empty object",
			data: customMarshaler{payload: `{}`},
			want: []byte(links),
		},
		{
			name: "null",
			data: customMarshaler{payload: `null`},
			want: []byte(links),
		},
		{
			name:    "array",
			data:    customMarshaler{payload: `[1,2]`},
			wantErr: true,
		},
		{
			name:    "scalar string",
			data:    customMarshaler{payload: `"text"`},
			wantErr: true,
		},
		{
			name:    "scalar number",
			data:    customMarshaler{payload: `42`},
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			data:    customMarshaler{payload: `not json`},
			wantErr: true,
		},
		{
			name:      "marshaler error",
			data:      customMarshaler{err: errCustomMarshal},
			wantErr:   true,
			wantErrIs: errCustomMarshal,
		},
	}

	for _, enc := range encoders {
		t.Run(enc.name, func(t *testing.T) {
			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					env := New().WrapRaw(tc.data)
					env.AddLink(Link{Rel: "self", Href: "/x"})

					b, err := enc.marshal(env)
					if tc.wantErr {
						if err == nil {
							t.Fatalf("expected error, got %s", b)
						}
						if len(b) != 0 {
							t.Fatalf("error returned partial output: %s", b)
						}
						if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
							t.Fatalf("error %v does not wrap %v", err, tc.wantErrIs)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					assertNoDuplicateKeys(t, b)
					if !utf8.Valid(b) {
						t.Fatalf("output is not valid UTF-8: %q", b)
					}
					assertJSONEqual(t, b, tc.want)
				})
			}
		})
	}
}

func TestEncoderMatrix_SemanticallyEqual(t *testing.T) {
	type payload struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	env := New().WrapRaw(&payload{ID: 1, Name: "Ada"})
	env.AddLink(Link{Rel: "self", Href: "/users/1"})

	stdOut, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	goccyOut, err := goccyjson.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	assertJSONEqual(t, stdOut, goccyOut)
	assertNoDuplicateKeys(t, stdOut)
	assertNoDuplicateKeys(t, goccyOut)
}

func TestEncoderMatrix_HTMLIsEscaped(t *testing.T) {
	type payload struct {
		Text string `json:"text"`
	}

	env := New().WrapRaw(&payload{Text: `<a href="/">A & B</a>`})
	env.AddLink(Link{Rel: "self", Href: "/x"})

	encoders := []struct {
		name    string
		marshal func(any) ([]byte, error)
	}{
		{name: "encoding/json", marshal: json.Marshal},
		{name: "goccy/go-json", marshal: goccyjson.Marshal},
	}

	for _, enc := range encoders {
		t.Run(enc.name, func(t *testing.T) {
			b, err := enc.marshal(env)
			if err != nil {
				t.Fatal(err)
			}
			if !utf8.Valid(b) {
				t.Fatalf("output is not valid UTF-8: %q", b)
			}
			for _, escaped := range []string{`\u003c`, `\u003e`, `\u0026`} {
				if !strings.Contains(string(b), escaped) {
					t.Errorf("output %s does not contain %s", b, escaped)
				}
			}

			root, ok := decodeJSONValue(t, b).(map[string]any)
			if !ok {
				t.Fatal("document is not an object")
			}
			if got := root["text"]; got != `<a href="/">A & B</a>` {
				t.Errorf("text = %#v, want original string", got)
			}
		})
	}
}
