# Versioning & Release Process

go-hal uses **Semantic Versioning** (SemVer) as specified at [semver.org](https://semver.org/). The current version is **v1.0.0**.

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
	fmt.Println(hal.GetVersion())        // "v1.0.0"
	info := hal.GetVersionInfo()
	fmt.Printf("Major: %d, Minor: %d, Patch: %d\n", 
		info.Major, info.Minor, info.Patch)
}
```

## Automated Version Bumping

The `scripts/bump-version.sh` script automates version updates:

```bash
# Bump patch version (v1.0.0 → v1.0.1)
./scripts/bump-version.sh patch

# Bump minor version (v1.0.0 → v1.1.0)
./scripts/bump-version.sh minor

# Bump major version (v1.0.0 → v2.0.0)
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

The CI pipeline (`/.github/workflows/ci.yml`) enforces three quality checks:

### 1. Test Coverage ≥ 70%

All commits must maintain test coverage above 70%. The CI pipeline:
- Runs all tests with coverage profiling
- Calculates total coverage percentage
- **Fails** if coverage drops below 70%

Coverage is reported per-commit in the CI output.

### 2. Advisory Benchmark Comparison

Pull requests get an advisory benchmark comparison on the stable Go toolchain:

- The base and head revisions are benchmarked with
  `go test -run='^$' -bench=. -benchmem -cpu=1 -count=10 -benchtime=200ms ./...`
- `benchstat` compares the two result sets and posts the table to the workflow summary
- The job is advisory: it does not fail the build or block a merge
- Reviewers decide whether a measured change matters for the affected path

### 3. Code Quality Checks

The pipeline also runs:
- `gofmt` enforcement (code formatting)
- `golangci-lint` (linting)
- Race detector (`go test -race`)

## Release Checklist

When preparing a release:

1. ✓ Ensure all tests pass and coverage ≥ 70%
2. ✓ Ensure benchmarks show no regressions
3. ✓ Update `CHANGELOG.md` (if maintained) with changes
4. Run: `./scripts/bump-version.sh [major|minor|patch]`
5. Review the commit and tag: `git log -1 && git tag -l`
6. Push: `git push origin main && git push origin v<VERSION>`
7. Create GitHub Release from the tag (optional but recommended)

## Version History

| Version | Date | Notes |
|---------|------|-------|
| v1.0.0  | 2026-04-22 | First full release |

## How Versioning Works Under the Hood

- `version.go` contains the source of truth for the version
- Git tags (e.g., `v1.0.0`) mark releases
- The bump script updates both the Go code and creates the Git tag
- CI ensures that version changes are properly committed before merging
