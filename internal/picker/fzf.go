package picker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	sessionmodel "github.com/adriankarlen/yeet/internal/model"
)

const defaultPrompt = " "

func RunFZF(ctx context.Context, items []sessionmodel.Session, opts Options) (sessionmodel.Session, bool, error) {
	if len(items) == 0 {
		return sessionmodel.Session{}, false, nil
	}
	command := opts.FZFCommand
	if command == "" {
		command = "fzf"
	}
	if _, err := exec.LookPath(command); err != nil {
		return sessionmodel.Session{}, false, fmt.Errorf("fzf picker requires %q in PATH: %w", command, err)
	}
	cmd := exec.CommandContext(ctx, command, fzfArgs(opts)...)
	cmd.Stdin = strings.NewReader(fzfInput(items, opts.SeparatorAware))
	var selected bytes.Buffer
	cmd.Stdout = &selected
	cmd.Stderr = opts.Output
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && (exitErr.ExitCode() == 1 || exitErr.ExitCode() == 130) {
			return sessionmodel.Session{}, false, nil
		}
		return sessionmodel.Session{}, false, err
	}
	idx, ok := fzfSelectionIndex(selected.String(), len(items))
	if !ok {
		return sessionmodel.Session{}, false, fmt.Errorf("fzf returned invalid selection %q", strings.TrimSpace(selected.String()))
	}
	return items[idx], true, nil
}

func fzfArgs(opts Options) []string {
	prompt := opts.Prompt
	if prompt == "" {
		prompt = defaultPrompt
	}
	args := []string{
		"--ansi",
		"--prompt=" + prompt,
		"--delimiter=\t",
		"--with-nth=3",
	}
	if opts.Placeholder != "" {
		args = append(args, "--header="+opts.Placeholder)
	}

	// Preview toggleable side-by-side using field 2 (the directory path)
	previewCmd := `target="{2}"; [ -z "$target" ] && target="{3}"; if [ -d "$target" ]; then eza --all --git --icons=always --group-directories-first --color=always "$target"; else echo "Not a directory: $target"; fi`
	if opts.DefaultPreviewCommand != "" {
		// Replace {} placeholder safely with "{2}"
		custom := strings.ReplaceAll(opts.DefaultPreviewCommand, "{}", `"{2}"`)
		previewCmd = `target="{2}"; if [ -d "$target" ]; then ` + custom + `; fi`
	}

	args = append(args,
		"--preview="+previewCmd,
		"--preview-window=right:50%:hidden:wrap",
		"--bind=ctrl-o:toggle-preview",
	)
	return args
}

func fzfInput(items []sessionmodel.Session, separatorAware bool) string {
	var b strings.Builder
	for i, s := range items {
		targetPath := s.Path
		if targetPath == "" {
			targetPath = s.Name
		}
		label := FormatDisplay(s)
		_, _ = fmt.Fprintf(&b, "%d\t%s\t%s\n", i, targetPath, label)
	}
	return b.String()
}

func fzfSelectionIndex(output string, count int) (int, bool) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return 0, false
	}
	line := strings.Split(trimmed, "\n")[0]
	parts := strings.Split(line, "\t")
	if len(parts) == 0 {
		return 0, false
	}
	idx, err := strconv.Atoi(parts[0])
	if err != nil || idx < 0 || idx >= count {
		return 0, false
	}
	return idx, true
}
