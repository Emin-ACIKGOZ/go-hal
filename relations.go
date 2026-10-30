// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

// appendRelationValue adds a relation value with the HAL multiplicity rules:
// one value is an object, repeated values form an array. The same helper is
// used for _links and _embedded values so their shapes cannot drift.
func appendRelationValue(values map[string]any, rel string, val any) {
	if existing, ok := values[rel]; ok {
		if slice, isSlice := existing.([]any); isSlice {
			values[rel] = append(slice, val)
		} else {
			values[rel] = []any{existing, val}
		}
	} else {
		values[rel] = val
	}
}
