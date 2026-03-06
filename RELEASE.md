# Release Infrastructure

This document describes the automated release infrastructure for the axel project.

## Overview

The release process is fully automated using GitHub Actions and GoReleaser. When a new tag is pushed, the workflow automatically:

1. Builds binaries for all supported platforms
2. Generates checksums
3. Creates a GitHub release with auto-generated changelog
4. Triggers Go module reindex at proxy.golang.org

## Files Created

### `.github/workflows/release.yml`

GitHub Actions workflow that runs on tag pushes:

- Triggers on any tag push (`*`)
- Sets up Go with latest 1.x version
- Runs GoReleaser with `release --clean`
- Triggers Go module reindex
- Has `contents: write` permissions

### `.github/workflows/ci.yml`

Continuous Integration workflow that runs on pushes/PRs to main/master:

- Runs tests with coverage
- Runs golangci-lint
- Builds on multiple OS (Linux, macOS, Windows)
- Uploads coverage to Codecov (requires `CODECOV_TOKEN` secret)

### `.goreleaser.yml`

GoReleaser configuration with:

- **Platforms**: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64
- **Binary name**: axel
- **Ldflags**: `-s -w -X main.version={{.Version}}` (matches Makefile pattern)
- **Archives**: tar.gz for Unix, zip for Windows
- **Checksums**: SHA256
- **Changelog**: Conventional commits with grouped sections
- **Release**: Auto-generated GitHub release with installation instructions

## Usage

### Creating a Release

1. **Create and push a tag:**

   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

2. **Monitor the workflow:**
   - Go to Actions tab in GitHub
   - Watch the "Release" workflow
   - Check the created release

3. **Verify the release:**
   - Check the GitHub releases page
   - Verify binaries are attached
   - Verify changelog is generated

### Semantic Versioning

Use semantic versioning for tags:

- `v1.0.0` - Major release
- `v1.1.0` - Minor release (new features)
- `v1.1.1` - Patch release (bug fixes)

### Conventional Commits

Use conventional commit messages for better changelog generation:

- `feat:` - New features
- `fix:` - Bug fixes
- `perf:` - Performance improvements
- `refactor:` - Code refactoring
- `docs:` - Documentation changes
- `chore:` - Maintenance tasks
- `test:` - Test updates
- `ci:` - CI/CD changes

Breaking changes:

```
feat!: breaking change description
feat(scope)!: breaking change description
```

## Configuration Details

### Build Configuration

- **Main package**: `./cmd/scanner`
- **Binary name**: `axel`
- **Version injection**: via ldflags into `main.version`
- **CGO**: Disabled (static binaries)

### Archive Configuration

- **Unix systems**: tar.gz format
- **Windows**: zip format
- **Included files**: README.md, LICENSE

### Changelog Groups

The changelog is organized into sections:

1. 🚀 Features
2. 🐛 Bug Fixes
3. ⚡ Performance
4. ♻️ Refactoring
5. 📝 Documentation
6. 🔧 Maintenance
7. Others

Excluded from changelog:

- Commits containing "typo"

Grouped in separate sections (not excluded):

- `docs:` - Documentation changes
- `test:` - Test updates
- `ci:` - CI/CD changes

## CI Workflow

The CI workflow runs on:

- Push to `main` or `master` branches
- Pull requests to `main` or `master`

Jobs:

1. **test**: Runs tests with race detection and coverage
2. **lint**: Runs golangci-lint
3. **build**: Builds on Linux, macOS, and Windows

### Required Secrets

For full CI functionality, add these secrets to your repository:

- `CODECOV_TOKEN`: For coverage upload to Codecov

To add secrets:

1. Go to repository Settings → Secrets and variables → Actions
2. Click "New repository secret"
3. Add `CODECOV_TOKEN` with your Codecov token

## Local Testing

Test the GoReleaser configuration locally:

```bash
# Install GoReleaser
go install github.com/goreleaser/goreleaser/v2@latest

# Test the configuration (no release)
goreleaser release --snapshot --clean

# Check configuration
goreleaser check
```

## Troubleshooting

### Release Failed

1. Check the Actions log for errors
2. Verify the tag format (must start with 'v')
3. Ensure `GITHUB_TOKEN` has write permissions
4. Check `.goreleaser.yml` syntax

### Missing Binaries

1. Verify build matrix in `.goreleaser.yml`
2. Check Go version compatibility
3. Review build logs for errors

### Changelog Issues

1. Use conventional commit messages
2. Check commit message format
3. Review excluded patterns in config

## Additional Resources

- [GoReleaser Documentation](https://goreleaser.com/)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [Semantic Versioning](https://semver.org/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
