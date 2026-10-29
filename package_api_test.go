// Copyright (c) 2026 Emin Salih Açıkgöz
// SPDX-License-Identifier: MIT

package hal

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

type packageAPIUser struct {
	ID int `json:"id"`
}

func useDefaultInstance(t *testing.T) *Instance {
	t.Helper()
	old := DefaultInstance
	inst := New()
	DefaultInstance = inst
	t.Cleanup(func() { DefaultInstance = old })
	return inst
}

func TestPackageAPI_RegisterWrapAndCollection(t *testing.T) {
	useDefaultInstance(t)
	Register(func(_ context.Context, u *packageAPIUser) []Link {
		return []Link{{Rel: "self", Href: fmt.Sprintf("/users/%d", u.ID)}}
	})

	env := Wrap(context.Background(), &packageAPIUser{ID: 1})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)
	assertJSONEqual(t, b, []byte(`{"id":1,"_links":{"self":{"href":"/users/1"}}}`))

	page := Collection(
		context.Background(),
		[]*packageAPIUser{{ID: 1}, {ID: 2}},
		2,
		Link{Rel: "self", Href: "/users"},
	)
	b, err = json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	root, ok := decodeJSONValue(t, b).(map[string]any)
	if !ok {
		t.Fatal("collection document is not an object")
	}
	if got := fmt.Sprint(root["count"]); got != "2" {
		t.Errorf("count = %s, want 2", got)
	}
	embedded, ok := root["_embedded"].(map[string]any)
	if !ok {
		t.Fatalf("_embedded has type %T, want object", root["_embedded"])
	}
	items, ok := embedded["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("_embedded.items = %#v, want 2 items", embedded["items"])
	}
	for i, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("item %d has type %T, want object", i, raw)
		}
		self := relationObjects(t, item, "self")
		wantHref := fmt.Sprintf("/users/%d", i+1)
		if len(self) != 1 || self[0]["href"] != wantHref {
			t.Errorf("item %d self = %#v, want %q", i, self, wantHref)
		}
	}
}

func TestPackageAPI_RegisterCurie(t *testing.T) {
	useDefaultInstance(t)
	RegisterCurie("acme", "https://docs.example.com/rels/{rel}")
	Register(func(_ context.Context, _ *packageAPIUser) []Link {
		return []Link{{Rel: "acme:widget", Href: "/widgets/1"}}
	})

	env := Wrap(context.Background(), &packageAPIUser{ID: 1})
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	assertNoDuplicateKeys(t, b)

	root, ok := decodeJSONValue(t, b).(map[string]any)
	if !ok {
		t.Fatal("document is not an object")
	}
	curies := relationObjects(t, root, "curies")
	if len(curies) != 1 {
		t.Fatalf("got %d curies, want 1", len(curies))
	}
	if got := curies[0]["name"]; got != "acme" {
		t.Errorf("curies[0].name = %#v, want acme", got)
	}
	if len(relationObjects(t, root, "acme:widget")) != 1 {
		t.Error("CURIE link relation was lost")
	}
}

func TestVersion_Formatting(t *testing.T) {
	old := versionInfo
	t.Cleanup(func() { versionInfo = old })

	versionInfo.Major = 2
	versionInfo.Minor = 3
	versionInfo.Patch = 4
	versionInfo.PreRelease = ""

	if got := GetVersion(); got != "v2.3.4" {
		t.Fatalf("GetVersion() = %q, want v2.3.4", got)
	}
	info := GetVersionInfo()
	if info.Major != 2 || info.Minor != 3 || info.Patch != 4 || info.PreRelease != "" {
		t.Fatalf("GetVersionInfo() = %#v, want 2.3.4 without prerelease", info)
	}

	versionInfo.PreRelease = "rc.1"
	if got := GetVersion(); got != "v2.3.4-rc.1" {
		t.Fatalf("GetVersion() = %q, want v2.3.4-rc.1", got)
	}
	info = GetVersionInfo()
	if info.PreRelease != "rc.1" {
		t.Fatalf("GetVersionInfo().PreRelease = %q, want rc.1", info.PreRelease)
	}
}

func TestVersion_CurrentIsSemanticVersion(t *testing.T) {
	info := GetVersionInfo()
	if info.Major < 0 || info.Minor < 0 || info.Patch < 0 {
		t.Fatalf("negative version component: %#v", info)
	}

	want := fmt.Sprintf("v%d.%d.%d", info.Major, info.Minor, info.Patch)
	if info.PreRelease != "" {
		want += "-" + info.PreRelease
	}
	if got := GetVersion(); got != want {
		t.Fatalf("GetVersion() = %q, want %q from GetVersionInfo()", got, want)
	}
}
