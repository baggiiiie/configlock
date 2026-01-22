# Release Guide

This document explains how to create a new release of ConfigLock.

## Quick Release Process

### 1. Prepare the Release

Ensure all changes are committed and tests pass:
```bash
cd configlock
go test ./...
go build -o configlock
```

### 2. Create a Release on GitHub

**Option A: Using GitHub CLI (recommended)**
```bash
# Create a tag and release in one command
gh release create configlock-v1.0.0 --generate-notes --title "ConfigLock v1.0.0"
```

**Option B: Via GitHub Web UI**
1. Go to https://github.com/baggiiiie/configlock/releases/new
2. Click "Choose a tag" and type: `configlock-v1.0.0`
3. Click "Create new tag on publish"
4. Set title: `ConfigLock v1.0.0`
5. Click "Generate release notes" (optional)
6. Click "Publish release"

### 3. Automated Build

GitHub Actions will automatically:
1. Build binaries for all platforms:
   - `configlock-linux-amd64`
   - `configlock-linux-arm64`
   - `configlock-darwin-amd64`
   - `configlock-darwin-arm64`

2. Upload all binaries to the release

### 4. Verify the Release

After the workflow completes:

1. Visit the [releases page](https://github.com/baggiiiie/configlock/releases)
2. Verify all 4 binaries are attached
3. Test the install script:
   ```bash
   curl -sSL https://raw.githubusercontent.com/baggiiiie/configlock/main/configlock/install.sh | bash
   ```

## Manual Release (Alternative)

If you need to create a release manually without GitHub Actions:

### 1. Build All Binaries

```bash
cd configlock
VERSION="1.0.0"

# Build for all platforms
for GOOS in linux darwin; do
  for GOARCH in amd64 arm64; do
    OUTPUT="configlock-${GOOS}-${GOARCH}"
    echo "Building ${OUTPUT}..."
    CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
      -ldflags "-s -w -X github.com/baggiiiie/configlock/cmd.version=${VERSION}" \
      -trimpath \
      -o "${OUTPUT}"
  done
done

# Generate checksums
shasum -a 256 configlock-* > checksums.txt
```

### 2. Create GitHub Release Manually

1. Go to https://github.com/baggiiiie/configlock/releases/new
2. Create a tag: `configlock-v1.0.0`
3. Set title: `ConfigLock v1.0.0`
4. Upload all files:
   - configlock-linux-amd64
   - configlock-linux-arm64
   - configlock-darwin-amd64
   - configlock-darwin-arm64
   - checksums.txt
5. Publish the release

## Version Numbering

Follow [Semantic Versioning](https://semver.org/):

- **MAJOR** version (1.x.x): Incompatible API changes
- **MINOR** version (x.1.x): Add functionality in a backward compatible manner
- **PATCH** version (x.x.1): Backward compatible bug fixes

Examples:
- `configlock-v1.0.0` - Initial release
- `configlock-v1.1.0` - New feature added
- `configlock-v1.1.1` - Bug fix
- `configlock-v2.0.0` - Breaking changes

## Testing a Release

Before pushing a tag, test the build locally:

```bash
# Build for current platform
go build -o configlock

# Test the binary
./configlock --version
./configlock --help

# Or test a cross-platform build
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -trimpath -o configlock-test
./configlock-test --version
```

## Rollback a Release

If you need to delete a bad release:

```bash
# Using GitHub CLI
gh release delete configlock-v1.0.0 --yes

# Or delete manually from the web UI at:
# https://github.com/baggiiiie/configlock/releases

# Also delete the tag
git push --delete origin configlock-v1.0.0
git tag -d configlock-v1.0.0
```

Then create a new corrected release.

## Troubleshooting

### GitHub Actions fails

1. Check the [Actions tab](https://github.com/baggiiiie/configlock/actions)
2. Review the workflow logs
3. Common issues:
   - Go version mismatch
   - Missing dependencies
   - Build errors in code

### Install script fails

1. Verify the release exists and has all binaries
2. Check that binary names match: `configlock-{os}-{arch}`
3. Test the download URL manually:
   ```bash
   curl -I https://github.com/baggiiiie/configlock/releases/download/configlock-v1.0.0/configlock-linux-amd64
   ```

## Checklist

Before creating a release:

- [ ] All tests pass: `go test ./...`
- [ ] Version number follows semver
- [ ] CHANGELOG updated (if exists)
- [ ] Build works locally: `make build-all`
- [ ] Tag follows format: `configlock-v*.*.*`
- [ ] Committed all changes to main branch
