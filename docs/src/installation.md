# Installation

## From source (go install)

The simplest way to install shanty:

```bash
go install github.com/intigraph/shanty/cmd/shanty@latest
```

This places the binary in your `$GOPATH/bin` (usually `~/go/bin`). Make sure this directory is in your `PATH`.

## Build from source

Clone the repository and build:

```bash
git clone https://github.com/intigraph/shanty.git
cd shanty
just build
```

Or without just:

```bash
go build -o shanty ./cmd/shanty
```

To install to `~/.local/bin`:

```bash
just install
```

## Binary releases

Pre-built binaries for Linux and macOS (amd64 and arm64) are available on the [Releases](https://github.com/intigraph/shanty/releases) page.

Download the appropriate archive, extract it, and place the `shanty` binary somewhere in your `PATH`.

## Runtime dependency

Shanty requires **tmux** to be installed. Install it with your system package manager:

```bash
# Debian/Ubuntu
sudo apt install tmux

# macOS
brew install tmux

# Arch
sudo pacman -S tmux
```

## Verify installation

```bash
shanty --version
```
