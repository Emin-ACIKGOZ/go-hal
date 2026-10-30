// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
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
		data      any
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
			name: "nil interface",
			data: nil,
			want: []byte(links),
		},
		{
			name: "typed nil marshaler",
			data: (*customMarshaler)(nil),
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
			name:    "invalid UTF-8",
			data:    customMarshaler{payload: "{\"id\":\"\xff\"}"},
			wantErr: true,
		},
		{
			name:      "reserved field conflict",
			data:      customMarshaler{payload: `{"id":1,"_links":{}}`},
			wantErr:   true,
			wantErrIs: ErrHALFieldConflict,
		},
		{
			name:      "marshaler error",
			data:      customMarshaler{err: errCustomMarshal},
			wantErr:   true,
			wantErrIs: errCustomMarshal,
		},
	}

	wrappers := []struct {
		name string
		wrap func(any) *Envelope
	}{
		{
			name: "generated",
			wrap: func(data any) *Envelope {
				env := New().WrapRaw(data)
				env.AddLink(Link{Rel: "self", Href: "/x"})
				return env
			},
		},
		{
			name: "precomputed",
			wrap: func(data any) *Envelope {
				return New().WrapPrecomputed(context.Background(), data, []byte(`{"self":{"href":"/x"}}`))
			},
		},
	}

	for _, wrapper := range wrappers {
		t.Run(wrapper.name, func(t *testing.T) {
			for _, enc := range encoders {
				t.Run(enc.name, func(t *testing.T) {
					for _, tc := range tests {
						t.Run(tc.name, func(t *testing.T) {
							env := wrapper.wrap(tc.data)

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
		})
	}
}

func TestEncoderMatrix_DuplicateKeysStrict(t *testing.T) {
	encoders := []struct {
		name    string
		marshal func(any) ([]byte, error)
	}{
		{name: "encoding/json", marshal: json.Marshal},
		{name: "goccy/go-json", marshal: goccyjson.Marshal},
	}

	tests := []struct {
		name    string
		payload string
		wantErr bool
	}{
		{name: "clean object", payload: `{"id":1}`},
		{name: "duplicate top-level key", payload: `{"id":1,"id":2}`, wantErr: true},
		{name: "duplicate nested key", payload: `{"outer":{"id":1,"id":2}}`, wantErr: true},
	}

	wrappers := []struct {
		name string
		wrap func(any) *Envelope
	}{
		{
			name: "generated",
			wrap: func(data any) *Envelope {
				env := New(WithStrictJSON()).WrapRaw(data)
				env.AddLink(Link{Rel: "self", Href: "/x"})
				return env
			},
		},
		{
			name: "precomputed",
			wrap: func(data any) *Envelope {
				return New(WithStrictJSON()).WrapPrecomputed(context.Background(), data, []byte(`{"self":{"href":"/x"}}`))
			},
		},
	}

	for _, wrapper := range wrappers {
		t.Run(wrapper.name, func(t *testing.T) {
			for _, enc := range encoders {
				t.Run(enc.name, func(t *testing.T) {
					for _, tc := range tests {
						t.Run(tc.name, func(t *testing.T) {
							env := wrapper.wrap(customMarshaler{payload: tc.payload})

							b, err := enc.marshal(env)
							if tc.wantErr {
								if !errors.Is(err, ErrDuplicateJSONKey) {
									t.Fatalf("error = %v (output %s), want ErrDuplicateJSONKey", err, b)
								}
								return
							}
							if err != nil {
								t.Fatal(err)
							}
							assertNoDuplicateKeys(t, b)
						})
					}
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
