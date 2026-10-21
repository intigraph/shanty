# Shanty — terminal multiplexer with Dracula theme
# https://github.com/intigraph/shanty

binary := "shanty"
install_dir := env("HOME") / ".local/bin"

version := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
commit := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
build_time := `date -u +"%Y-%m-%dT%H:%M:%SZ"`

ldflags := "-X github.com/intigraph/shanty/internal/cmd.Version=" + version + " -X github.com/intigraph/shanty/internal/cmd.Commit=" + commit + " -X github.com/intigraph/shanty/internal/cmd.BuildTime=" + build_time

# Build the shanty binary
build:
    go build -ldflags "{{ldflags}}" -o {{binary}} ./cmd/shanty

# Install to ~/.local/bin
#
# Temp file + rename, not a plain cp: cp writes through to the existing inode and
# the kernel refuses that for a running binary ("Text file busy"). The status bar
# re-runs `shanty seg` in every pane, so on a host using shanty that is the normal
# state. A rename swaps the directory entry; running processes keep the old inode.
install: build
    mkdir -p {{install_dir}}
    cp {{binary}} {{install_dir}}/.{{binary}}.new
    mv -f {{install_dir}}/.{{binary}}.new {{install_dir}}/{{binary}}
    @echo "Installed {{binary}} to {{install_dir}}"

# Run all tests
test:
    go test ./...

# Run tests with verbose output
test-v:
    go test -v ./...

# Run linter
lint:
    go vet ./...

# Format code
fmt:
    gofmt -s -w .

# Check formatting (CI-friendly, fails on diff)
fmt-check:
    @test -z "$(gofmt -s -l .)" || (echo "Files need formatting:"; gofmt -s -l .; exit 1)

# Run all checks (test + lint + fmt)
check: fmt-check lint test

# Clean build artifacts
clean:
    rm -f {{binary}}

# Build for all platforms (release)
release:
    goreleaser release --snapshot --clean

# Show version info
version:
    @echo "version: {{version}}"
    @echo "commit:  {{commit}}"
    @echo "built:   {{build_time}}"
