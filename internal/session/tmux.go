package session

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/intigraph/shanty/internal/config"
)

// GenerateConfig creates a shanty tmux configuration file and returns its path.
func GenerateConfig() (string, error) {
	theme := config.DefaultTheme()

	// Try loading theme from file
	if path := config.FindThemeFile(); path != "" {
		if t, err := config.LoadTheme(path); err == nil {
			theme = t
		}
	}

	confDir, err := configDir()
	if err != nil {
		return "", err
	}

	confPath := filepath.Join(confDir, "tmux.conf")
	f, err := os.Create(confPath)
	if err != nil {
		return "", fmt.Errorf("creating tmux config: %w", err)
	}
	defer f.Close()

	// Header
	fmt.Fprintln(f, "# Shanty — generated tmux configuration")
	fmt.Fprintln(f, "# Theme:", theme.Name)
	fmt.Fprintln(f)

	// Prefix key: ctrl-a (byobu convention)
	fmt.Fprintln(f, "# Prefix key")
	fmt.Fprintln(f, "unbind-key C-b")
	fmt.Fprintln(f, "set-option -g prefix C-a")
	fmt.Fprintln(f)

	// Terminal settings
	fmt.Fprintln(f, "# Terminal")
	fmt.Fprintln(f, "set-option -g default-terminal 'tmux-256color'")
	fmt.Fprintln(f, "set-option -ga terminal-overrides ',xterm-256color:Tc'")
	fmt.Fprintln(f, "set-option -g mouse on")
	fmt.Fprintln(f, "set-option -g base-index 1")
	fmt.Fprintln(f, "set-option -g history-limit 50000")
	fmt.Fprintln(f)

	// Theme colors
	fmt.Fprintln(f, "# Pane borders")
	fmt.Fprintf(f, "set-option -g pane-active-border-style 'fg=%s'\n", theme.ActiveBorder)
	fmt.Fprintf(f, "set-option -g pane-border-style 'fg=%s'\n", theme.InactiveBorder)
	fmt.Fprintln(f)

	// Message styling
	fmt.Fprintf(f, "set-option -g message-style 'fg=%s,bg=%s'\n", theme.FG, theme.StatusBG)
	fmt.Fprintln(f)

	// Keybindings
	fmt.Fprintln(f, config.RenderKeybindings(config.DefaultKeybindings()))

	// Status bar
	fmt.Fprintln(f, config.RenderStatusBar(theme, config.DefaultStatusBar()))

	return confPath, nil
}

func configDir() (string, error) { return config.Dir() }
