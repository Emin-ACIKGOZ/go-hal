// Copyright (c) 2025 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// TestData is the struct we'll use for benchmarks
type TestData struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

// ManualHALResponse represents manually constructed HAL (what users would do without the library)
type ManualHALResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
	Links struct {
		Self struct {
			Href string `json:"href"`
		} `json:"self"`
	} `json:"_links"`
}

// init registers the HAL generator for TestData once
func init() {
	Register(func(_ context.Context, t *TestData) []Link {
		return []Link{
			{Rel: "self", Href: fmt.Sprintf("/users/%d", t.ID)},
		}
	})
}

// --- Benchmark: go-hal ---

func BenchmarkHAL_Wrap_And_Marshal(b *testing.B) {
	ctx := context.Background()
	data := &TestData{
		ID:    42,
		Name:  "Alice Johnson",
		Email: "alice@example.com",
		Age:   30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		envelope := Wrap(ctx, data)
		_, _ = json.Marshal(envelope)
	}
}

func BenchmarkHAL_Marshal_Only(b *testing.B) {
	ctx := context.Background()
	data := &TestData{
		ID:    42,
		Name:  "Alice Johnson",
		Email: "alice@example.com",
		Age:   30,
	}

	envelope := Wrap(ctx, data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(envelope)
	}
}

// --- Benchmark: Manual HAL (standard approach) ---

func BenchmarkManual_HAL(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := ManualHALResponse{
			ID:    42,
			Name:  "Alice Johnson",
			Email: "alice@example.com",
			Age:   30,
		}
		response.Links.Self.Href = fmt.Sprintf("/users/%d", response.ID)
		_, _ = json.Marshal(response)
	}
}

// --- Benchmark: Raw data without HAL ---

func BenchmarkRaw_JSON_No_HAL(b *testing.B) {
	data := &TestData{
		ID:    42,
		Name:  "Alice Johnson",
		Email: "alice@example.com",
		Age:   30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(data)
	}
}

// --- Benchmark: Reflection-based generator (simulate what users might do) ---

func BenchmarkReflection_Based_HAL(b *testing.B) {
	ctx := context.Background()
	instance := New()

	// Register using reflection (the old way that's slower)
	instance.RegisterInstance(func(_ context.Context, t *TestData) []Link {
		return []Link{
			{Rel: "self", Href: fmt.Sprintf("/users/%d", t.ID)},
		}
	})

	data := &TestData{
		ID:    42,
		Name:  "Alice Johnson",
		Email: "alice@example.com",
		Age:   30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		envelope := instance.Wrap(ctx, data)
		_, _ = json.Marshal(envelope)
	}
}

// --- Benchmark: Multiple links (realistic scenario) ---

func BenchmarkHAL_Multiple_Links(b *testing.B) {
	ctx := context.Background()
	instance := New()

	instance.RegisterInstance(func(_ context.Context, t *TestData) []Link {
		return []Link{
			{Rel: "self", Href: fmt.Sprintf("/users/%d", t.ID)},
			{Rel: "edit", Href: fmt.Sprintf("/users/%d/edit", t.ID)},
			{Rel: "delete", Href: fmt.Sprintf("/users/%d", t.ID), Method: "DELETE"},
			{Rel: "list", Href: "/users"},
		}
	})

	data := &TestData{
		ID:    42,
		Name:  "Alice Johnson",
		Email: "alice@example.com",
		Age:   30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		envelope := instance.Wrap(ctx, data)
		_, _ = json.Marshal(envelope)
	}
}

func BenchmarkManual_Multiple_Links(b *testing.B) {
	type ManualResponse struct {
		ID    int    `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
		Links struct {
			Self struct {
				Href string `json:"href"`
			} `json:"self"`
			Edit struct {
				Href string `json:"href"`
			} `json:"edit"`
			Delete struct {
				Href   string `json:"href"`
				Method string `json:"method"`
			} `json:"delete"`
			List struct {
				Href string `json:"href"`
			} `json:"list"`
		} `json:"_links"`
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := ManualResponse{
			ID:    42,
			Name:  "Alice Johnson",
			Email: "alice@example.com",
			Age:   30,
		}
		response.Links.Self.Href = fmt.Sprintf("/users/%d", response.ID)
		response.Links.Edit.Href = fmt.Sprintf("/users/%d/edit", response.ID)
		response.Links.Delete.Href = fmt.Sprintf("/users/%d", response.ID)
		response.Links.Delete.Method = "DELETE"
		response.Links.List.Href = "/users"
		_, _ = json.Marshal(response)
	}
}

// --- Benchmark: Byte buffer encoder (efficient baseline) ---

func BenchmarkByteBuffer_Encoder(b *testing.B) {
	ctx := context.Background()
	instance := New()

	instance.RegisterInstance(func(_ context.Context, t *TestData) []Link {
		return []Link{
			{Rel: "self", Href: fmt.Sprintf("/users/%d", t.ID)},
		}
	})

	data := &TestData{
		ID:    42,
		Name:  "Alice Johnson",
		Email: "alice@example.com",
		Age:   30,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		envelope := instance.Wrap(ctx, data)
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		_ = enc.Encode(envelope)
	}
}

// --- Memory allocation benchmarks (allocs/op) ---

func BenchmarkAllocs_HAL_Wrap_And_Marshal(b *testing.B) {
	ctx := context.Background()
	data := &TestData{
		ID:    42,
		Name:  "Alice Johnson",
		Email: "alice@example.com",
		Age:   30,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		envelope := Wrap(ctx, data)
		_, _ = json.Marshal(envelope)
	}
}

func BenchmarkAllocs_Manual_HAL(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response := ManualHALResponse{
			ID:    42,
			Name:  "Alice Johnson",
			Email: "alice@example.com",
			Age:   30,
		}
		response.Links.Self.Href = fmt.Sprintf("/users/%d", response.ID)
		_, _ = json.Marshal(response)
	}
}

func BenchmarkAllocs_Raw_JSON(b *testing.B) {
	data := &TestData{
		ID:    42,
		Name:  "Alice Johnson",
		Email: "alice@example.com",
		Age:   30,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(data)
	}
}

// --- Stress test: Large result set ---

func BenchmarkHAL_Large_Collection(b *testing.B) {
	ctx := context.Background()
	instance := New()

	instance.RegisterInstance(func(_ context.Context, t *TestData) []Link {
		return []Link{
			{Rel: "self", Href: fmt.Sprintf("/users/%d", t.ID)},
		}
	})

	// Simulate a collection with many items
	collection := make([]*TestData, 100)
	for i := 0; i < 100; i++ {
		collection[i] = &TestData{
			ID:    i,
			Name:  fmt.Sprintf("User %d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
			Age:   20 + i%50,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Marshal each item separately (typical REST API pattern)
		for _, item := range collection {
			envelope := instance.Wrap(ctx, item)
			_, _ = json.Marshal(envelope)
		}
	}
}

func BenchmarkManual_Large_Collection(b *testing.B) {
	collection := make([]*TestData, 100)
	for i := 0; i < 100; i++ {
		collection[i] = &TestData{
			ID:    i,
			Name:  fmt.Sprintf("User %d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
			Age:   20 + i%50,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, item := range collection {
			response := ManualHALResponse{
				ID:    item.ID,
				Name:  item.Name,
				Email: item.Email,
				Age:   item.Age,
			}
			response.Links.Self.Href = fmt.Sprintf("/users/%d", item.ID)
			_, _ = json.Marshal(response)
		}
	}
}
