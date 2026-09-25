package picker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	sessionmodel "github.com/adriankarlen/yeet/internal/model"
)

// FormatDisplay renders a clean line matching sesh list -i:
// icon + name/path formatted cleanly.
func FormatDisplay(s sessionmodel.Session) string {
	var icon string
	switch s.Source {
	case "herdr":
		// Tmux/herdr window/workspace icon: \uf2d0  or \uebc8 or \uf0cc6 or similar
		// In sesh tmux windows use  or ◫
		icon = "\x1b[36m\x1b[39m"
	case "config":
		icon = "\x1b[90m\x1b[39m"
	default:
		// zoxide or directory
		icon = "\x1b[36m\x1b[39m"
	}

	displayPath := s.Path
	if displayPath == "" {
		displayPath = s.Name
	} else {
		displayPath = abbreviateHome(displayPath)
	}

	return fmt.Sprintf("%s %s", icon, displayPath)
}

func abbreviateHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}

// Run executes the minimal picker.
// It uses gum filter by default, reading global GUM_* environment variables.
func Run(items []sessionmodel.Session, opts Options) (sessionmodel.Session, bool, error) {
	if len(items) == 0 {
		return sessionmodel.Session{}, false, nil
	}

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	gumCmd := opts.GumCommand
	if gumCmd == "" {
		gumCmd = "gum"
	}

	if _, err := exec.LookPath(gumCmd); err != nil {
		// Fallback to fzf if gum is not available
		return RunFZF(ctx, items, opts)
	}

	prompt := opts.Prompt
	if prompt == "" {
		prompt = " "
	}
	placeholder := opts.Placeholder
	if placeholder == "" {
		placeholder = "Pick a sesh"
	}

	args := []string{
		"filter",
		"--limit", "1",
		"--no-sort",
		"--fuzzy",
		"--no-strip-ansi",
		"--placeholder", placeholder,
		"--prompt", prompt,
	}

	// Prepare map of formatted line -> session
	var lines []string
	itemByLine := make(map[string]sessionmodel.Session, len(items))
	for _, item := range items {
		disp := FormatDisplay(item)
		// Ensure unique lines for lookup
		lookupKey := stripAnsi(disp)
		lines = append(lines, disp)
		if _, exists := itemByLine[lookupKey]; !exists {
			itemByLine[lookupKey] = item
		}
	}

	inputData := strings.Join(lines, "\n") + "\n"

	cmd := exec.CommandContext(ctx, gumCmd, args...)
	cmd.Stdin = strings.NewReader(inputData)

	// We want gum to output to a pipe so we capture the selection
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return sessionmodel.Session{}, false, err
	}
	cmd.Stderr = opts.Output
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return sessionmodel.Session{}, false, err
	}

	outBytes, readErr := io.ReadAll(stdoutPipe)
	waitErr := cmd.Wait()

	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) && (exitErr.ExitCode() == 1 || exitErr.ExitCode() == 130) {
			return sessionmodel.Session{}, false, nil
		}
		return sessionmodel.Session{}, false, waitErr
	}
	if readErr != nil {
		return sessionmodel.Session{}, false, readErr
	}

	selectedStr := strings.TrimSpace(string(outBytes))
	if selectedStr == "" {
		return sessionmodel.Session{}, false, nil
	}

	cleanSelected := stripAnsi(selectedStr)
	if s, ok := itemByLine[cleanSelected]; ok {
		return s, true, nil
	}

	// Fallback: match by suffix / substring
	for line, s := range itemByLine {
		if strings.Contains(line, cleanSelected) || strings.Contains(cleanSelected, line) {
			return s, true, nil
		}
	}

	return sessionmodel.Session{}, false, fmt.Errorf("could not resolve selection: %q", cleanSelected)
}

func stripAnsi(s string) string {
	var b strings.Builder
	inEsc := false
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEsc = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return strings.TrimSpace(b.String())
}
