// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import "testing"

func TestDuplicateScanner_NumbersAndEOF(t *testing.T) {
	valid := []string{
		`1e1000`,
		`-1e1000`,
		`123456789012345678901234567890`,
		`{"n":1e1000}`,
		`[1e1000,{"n":123456789012345678901234567890}]`,
		`null`,
		`{}`,
		`[]`,
	}
	for _, data := range valid {
		t.Run("valid "+data, func(t *testing.T) {
			if err := findDuplicateJSONKeys([]byte(data)); err != nil {
				t.Fatalf("valid JSON rejected: %v", err)
			}
		})
	}

	invalid := []struct {
		name string
		data string
	}{
		{name: "empty", data: ""},
		{name: "whitespace", data: " \n\t"},
		{name: "truncated object", data: `{"a":1`},
		{name: "truncated array", data: `[{"a":1}`},
		{name: "trailing object", data: `{"a":1}{"b":2}`},
		{name: "trailing number", data: `1 2`},
		{name: "trailing garbage", data: `{"a":1} x`},
		{name: "duplicate top level", data: `{"a":1,"a":2}`},
		{name: "duplicate escaped key", data: `{"a":1,"\u0061":2}`},
		{name: "duplicate nested", data: `{"outer":{"a":1,"a":2}}`},
		{name: "duplicate in array", data: `[{"a":1,"a":2}]`},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if err := findDuplicateJSONKeys([]byte(tc.data)); err == nil {
				t.Fatalf("invalid JSON accepted: %s", tc.data)
			}
		})
	}
}
