// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

// Link represents a HAL Hypermedia link.
type Link struct {
	Rel       string `json:"-"` // Used for map keys, not serialized directly
	Href      string `json:"href"`
	Templated bool   `json:"templated,omitempty"`
	Type      string `json:"type,omitempty"`
	Title     string `json:"title,omitempty"`
	Name      string `json:"name,omitempty"`
	Method    string `json:"method,omitempty"` // Non-standard but common hint
}

// Envelope is the container for your data + HAL metadata.
type Envelope struct {
	Data     any            // The user's struct
	instance *Instance      // The registry instance to use
	links    map[string]any // Computed during marshal
	embedded map[string]any // Computed during marshal
}

// InstanceOption configures a new HAL Instance.
type InstanceOption func(*Instance)

// WithStrictMode enables panic on generator mismatches (e.g. passing T instead of *T).
func WithStrictMode() InstanceOption {
	return func(i *Instance) {
		i.strictMode = true
	}
}
