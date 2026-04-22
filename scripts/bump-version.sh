#!/bin/bash
set -e

# bump-version.sh - Automated semantic versioning for go-hal
# Usage: ./scripts/bump-version.sh [major|minor|patch]
# Or without args, defaults to patch bump

bump_type="${1:-patch}"

if [[ ! "$bump_type" =~ ^(major|minor|patch)$ ]]; then
    echo "Error: bump_type must be 'major', 'minor', or 'patch'"
    echo "Usage: $0 [major|minor|patch]"
    exit 1
fi

# Extract current version from version.go
version_file="version.go"
current_version=$(grep -oP '(?<=Major:\s)\d+' "$version_file" | head -1)
current_minor=$(grep -oP '(?<=Minor:\s)\d+' "$version_file" | head -1)
current_patch=$(grep -oP '(?<=Patch:\s)\d+' "$version_file" | head -1)

major=$current_version
minor=$current_minor
patch=$current_patch

case "$bump_type" in
    major)
        ((major++))
        minor=0
        patch=0
        ;;
    minor)
        ((minor++))
        patch=0
        ;;
    patch)
        ((patch++))
        ;;
esac

new_version="v${major}.${minor}.${patch}"

# Update version.go
sed -i "s/Major:\s*[0-9]\+/Major:      $major/" "$version_file"
sed -i "s/Minor:\s*[0-9]\+/Minor:      $minor/" "$version_file"
sed -i "s/Patch:\s*[0-9]\+/Patch:      $patch/" "$version_file"

echo "Version bumped: v${current_version}.${current_minor}.${current_patch} -> $new_version"

# Verify version.go is valid Go code
if ! go fmt "$version_file" > /dev/null 2>&1; then
    echo "Error: Failed to format $version_file"
    exit 1
fi

if ! go build -o /dev/null 2>&1; then
    echo "Error: Build failed after version bump"
    exit 1
fi

# Commit the version bump
git add "$version_file"
git commit -m "chore: Bump version to $new_version"

# Create git tag
git tag -a "$new_version" -m "Release $new_version"

echo "✓ Version updated and committed"
echo "✓ Git tag created: $new_version"
echo ""
echo "To push the tag:"
echo "  git push origin $new_version"
