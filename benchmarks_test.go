package hal

import (
	"context"
	"fmt"
	"testing"

	json "github.com/goccy/go-json"
)

type BenchmarkUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Age   int    `json:"age"`
}

func init() {
	Register(func(_ context.Context, u *BenchmarkUser) []Link {
		return []Link{{Rel: "self", Href: fmt.Sprintf("/users/%d", u.ID)}}
	})
}

func BenchmarkBaseline(b *testing.B) {
	ctx := context.Background()
	u := &BenchmarkUser{ID: 42, Name: "Alice", Email: "alice@example.com", Age: 30}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env := Wrap(ctx, u)
		_, _ = json.Marshal(env)
	}
}

func BenchmarkWrapPrecomputed(b *testing.B) {
	inst := New()
	ctx := context.Background()
	u := &BenchmarkUser{ID: 42, Name: "Alice", Email: "alice@example.com", Age: 30}
	
	// Pre-computed links JSON: {"self":{"href":"/users/42"}}
	linksJSON := []byte(`{"self":{"href":"/users/42"}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env := inst.WrapPrecomputed(ctx, u, linksJSON)
		_, _ = json.Marshal(env)
	}
}

func BenchmarkRegisterStatic(b *testing.B) {
	inst := New()
	ctx := context.Background()
	
	// Register static links once (package-level function)
	RegisterStatic(inst, &BenchmarkUser{}, []Link{{Rel: "self", Href: "/users/42"}})
	
	u := &BenchmarkUser{ID: 42, Name: "Alice", Email: "alice@example.com", Age: 30}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env := inst.Wrap(ctx, u)  // Uses auto-detected precomputed
		_, _ = json.Marshal(env)
	}
}