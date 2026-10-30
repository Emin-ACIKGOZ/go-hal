# Versioning & Release Process

go-hal uses **Semantic Versioning** (SemVer) as specified at [semver.org](https://semver.org/). The current version is **v1.4.0**.

## Version Format

Versions follow the pattern: `v<MAJOR>.<MINOR>.<PATCH>[-PRERELEASE]`

- **MAJOR**: Incompatible API changes
- **MINOR**: New backwards-compatible functionality
- **PATCH**: Bug fixes and performance improvements
- **PRERELEASE** (optional): Alpha, beta, or RC versions (e.g., `v1.1.0-beta`)

## Accessing the Version

You can retrieve the current version programmatically:

```go
package main

import (
	"fmt"
	"github.com/Emin-ACIKGOZ/go-hal"
)

func main() {
	fmt.Println(hal.GetVersion())        // "v1.4.0"
	info := hal.GetVersionInfo()
	fmt.Printf("Major: %d, Minor: %d, Patch: %d\n", 
		info.Major, info.Minor, info.Patch)
}
```

## Automated Version Bumping

The `scripts/bump-version.sh` script automates version updates:

```bash
# Bump patch version (v1.4.0 → v1.4.1)
./scripts/bump-version.sh patch

# Bump minor version (v1.4.0 → v1.5.0)
./scripts/bump-version.sh minor

# Bump major version (v1.4.0 → v2.0.0)
./scripts/bump-version.sh major

# Default is patch
./scripts/bump-version.sh
```

The script will:
1. Update `version.go` with the new version
2. Verify the build still works
3. Commit the version change
4. Create an annotated Git tag
5. Print push instructions

After running the script, push the tag to trigger releases:

```bash
git push origin v1.0.1  # Push specific tag
# or
git push origin --tags  # Push all tags
```

## CI/CD Quality Gates

The CI pipeline (`/.github/workflows/ci.yml`) runs these gates:

### 1. Tests on Supported Toolchains

The test matrix uses Go `1.26.x` (the repository minimum) and `1.27.x` with
`GOTOOLCHAIN=local`. Each job runs:

- `go vet ./...`
- `go test -count=1 -shuffle=on ./...`
- `go test -count=1 -race -shuffle=on ./...`
- eight 5-second fuzz smoke targets

### 2. Test Coverage ≥ 70%

The coverage job:
- Runs all tests with a coverage profile
- Calculates total coverage with `awk`
- **Fails** if coverage drops below 70%

### 3. Code Quality

- `gofmt` enforcement (code formatting)
- `golangci-lint` v2.15.0 with `.golangci.yml`

### 4. Advisory Benchmark Comparison

Pull requests get an advisory benchmark comparison on the stable Go toolchain:

- The base and head revisions are benchmarked with
  `go test -run='^$' -bench=. -benchmem -cpu=1 -count=10 -benchtime=200ms ./...`
- `benchstat` compares the two result sets and posts the table to the workflow summary
- The job is advisory: it does not fail the build or block a merge
- Reviewers decide whether a measured change matters for the affected path

## Release Checklist

When preparing a release:

1. ✓ Ensure all tests, vet, race, and fuzz smoke checks pass, and coverage ≥ 70%
2. ✓ Review the advisory benchmark comparison for unexpected regressions
3. ✓ Update `CHANGELOG.md` with the release notes
4. Run: `./scripts/bump-version.sh [major|minor|patch]`
5. Review the commit and tag: `git log -1 && git tag -l`
6. Push: `git push origin main && git push origin v<VERSION>`
7. Create GitHub Release from the tag (optional but recommended)

## Release v1.4.0

v1.4.0 closes the remaining draft-11 gap-closure findings as a non-breaking
minor. Default output now derives `templated: true` for RFC 6570 template links
(dynamic, static, Collection, and struct-supplied curies) and generates
`curies` declarations for registered prefixes used only in `_embedded` keys,
including a byte-splice into precomputed parents. `ContentType` now requires
absolute profile URIs, and strict `Link.Profile` validation does the same.
Strict relations honor resource-local and inherited CURIE declarations, and
strict JSON applies document-wide to embedded child envelopes and raw embedded
resources. Client-side navigation remains out of scope as optional.

Migration: default output is additive only. `ContentType` rejects non-absolute
profiles. Strict links accept underived template structs because normalization
supplies the flag, while precomputed JSON still requires an explicit
`templated: true`. Default relation grammar and case-collision behavior are
unchanged.

## Release v1.3.0

v1.3.0 completes the server-side draft-11 feature set: arbitrary embedded
resources and the cache pattern, opt-in strict link/relation/CURIE/JSON
validation, stable link shape policies, media constants and `ContentType`, and
parent-fallback CURIE scope for embedded resources. Client-side `halclient`
remains deferred as optional. The public API is additive; the main default
behavior change is documented relation/CURIE-name hardening.

## Version History

| Version | Date | Notes |
|---------|------|-------|
| v1.4.0  | 2026-10-30 | Draft-11 gap closure; template derivation, embedded-only curies, scoped strict relations and JSON |
| v1.3.0  | 2026-10-30 | Full server-side HAL compliance; embedded resources, strict modes, media helpers |
| v1.2.0  | 2026-10-30 | Go 1.26 minimum, dependency and security refresh |
| v1.1.0  | 2026-10-30 | HAL compliance remediation and validated precomputed links |
| v1.0.0  | 2026-04-22 | First full release |

## How Versioning Works Under the Hood

- `version.go` contains the source of truth for the version
- Git tags (e.g., `v1.0.0`) mark releases
- The bump script updates both the Go code and creates the Git tag
- CI ensures that version changes are properly committed before merging
