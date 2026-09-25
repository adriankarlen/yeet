package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	clonepkg "github.com/adriankarlen/yeet/internal/clone"
	"github.com/adriankarlen/yeet/internal/config"
	connectpkg "github.com/adriankarlen/yeet/internal/connect"
	"github.com/adriankarlen/yeet/internal/herdr"
	"github.com/adriankarlen/yeet/internal/model"
	"github.com/adriankarlen/yeet/internal/namer"
	pickerpkg "github.com/adriankarlen/yeet/internal/picker"
	"github.com/adriankarlen/yeet/internal/preview"
	"github.com/adriankarlen/yeet/internal/sources"
	"github.com/adriankarlen/yeet/internal/state"
)

var Version = "dev"

type App struct {
	Out io.Writer
	Err io.Writer
}

func New() *App { return &App{Out: os.Stdout, Err: os.Stderr} }

func (a *App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return a.usage()
	}
	switch args[0] {
	case "--version", "version":
		_, err := fmt.Fprintf(a.Out, "yeet %s\n", Version)
		return err
	case "list":
		return a.list(ctx, args[1:])
	case "connect":
		return a.connect(ctx, args[1:])
	case "preview":
		return a.preview(ctx, args[1:])
	case "clone":
		return a.clone(ctx, args[1:])
	case "root":
		return a.root(ctx, args[1:])
	case "last":
		return a.last(ctx, args[1:])
	case "plugin":
		return a.plugin(ctx, args[1:])
	case "picker":
		return a.picker(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (a *App) usage() error {
	_, err := fmt.Fprintln(a.Out, "yeet list|connect|preview|clone|root|last|picker|plugin|--version")
	return err
}

func (a *App) warnf(format string, args ...any) {
	if a.Err == nil {
		return
	}
	_, _ = fmt.Fprintf(a.Err, "warning: "+format+"\n", args...)
}

func (a *App) loadConfig(path string) (config.Config, error) {
	cfg, _, err := config.Load(config.LoadOptions{Path: path, Warn: a.Err})
	return cfg, err
}

func (a *App) collect(ctx context.Context, cfg config.Config, target string) ([]model.Session, error) {
	hs := sources.HerdrWorkspaces{Client: herdr.NewCLIClient()}
	return a.collectFrom(ctx, cfg, target, hs)
}

func (a *App) collectAllowUnavailableHerdr(ctx context.Context, cfg config.Config, target string) ([]model.Session, error) {
	hs := sources.HerdrWorkspaces{Client: herdr.NewCLIClient()}
	return a.collectFrom(ctx, cfg, target, ignoreSource{hs})
}

type ignoreSource struct {
	sources.Source
}

func (s ignoreSource) List(ctx context.Context) (model.Sessions, error) {
	sess, err := s.Source.List(ctx)
	if err != nil {
		return model.NewSessions(), nil
	}
	return sess, nil
}

func (a *App) collectFrom(ctx context.Context, cfg config.Config, target string, hs sources.Source) ([]model.Session, error) {
	home, _ := os.UserHomeDir()
	srcs := []sources.Source{
		hs,
		sources.ConfigSessions{Config: cfg, Home: home},
		sources.Zoxide{},
	}
	if target != "" {
		srcs = append(srcs, sources.DirectPath{Path: target})
	}
	merged, err := sources.Merge(ctx, srcs, []string{"herdr", "config", "zoxide"}, cfg.Blacklist, false, true)
	if err != nil {
		return nil, err
	}
	sources.ApplyConfig(&merged, cfg, home)
	return merged.Ordered(), nil
}

func (a *App) list(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	jsonOut := fs.Bool("json", false, "")
	cfgPath := fs.String("config", "", "")
	icons := fs.Bool("icons", false, "")
	_ = fs.Bool("i", false, "") // shorthand
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := a.loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	sessions, err := a.collectAllowUnavailableHerdr(ctx, cfg, "")
	if err != nil {
		return err
	}
	if *jsonOut {
		enc := json.NewEncoder(a.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(sessions)
	}
	for _, s := range sessions {
		if *icons || fs.Lookup("i").Value.String() == "true" {
			_, err = fmt.Fprintln(a.Out, pickerpkg.FormatDisplay(s))
		} else {
			if s.Path != "" {
				_, err = fmt.Fprintf(a.Out, "%s\t%s\t%s\n", s.Source, s.Name, s.Path)
			} else {
				_, err = fmt.Fprintf(a.Out, "%s\t%s\n", s.Source, s.Name)
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *App) picker(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("picker", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	cfgPath := fs.String("config", "", "")
	fzfPicker := fs.Bool("fzf", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := a.loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	sessions, err := a.collectAllowUnavailableHerdr(ctx, cfg, "")
	if err != nil {
		return err
	}

	opts := pickerpkg.Options{
		Context:               ctx,
		Output:                a.Out,
		Prompt:                cfg.TUI.Prompt,
		Placeholder:           cfg.TUI.Placeholder,
		DefaultPreviewCommand: cfg.DefaultSessionConfig.PreviewCommand,
	}

	useFZF := *fzfPicker || strings.EqualFold(os.Getenv("HERDR_YEET_PICKER"), "fzf")
	var selected model.Session
	var ok bool

	if useFZF {
		selected, ok, err = pickerpkg.RunFZF(ctx, sessions, opts)
	} else {
		selected, ok, err = pickerpkg.Run(sessions, opts)
	}
	if err != nil || !ok {
		return err
	}

	currentWorkspaceID := os.Getenv("HERDR_WORKSPACE_ID")
	res, err := connectpkg.Connect(ctx, herdr.NewCLIClient(), []model.Session{selected}, pickerTarget(selected), connectpkg.Options{
		Namer: func(ctx context.Context, p string) string { return namer.Namer{}.Name(ctx, p, cfg.DirLength) },
	})
	if err != nil {
		return err
	}
	a.recordWorkspaceSwitch(currentWorkspaceID, res.Session.WorkspaceID)
	return nil
}

func pickerTarget(s model.Session) string {
	if s.WorkspaceID != "" {
		return s.WorkspaceID
	}
	if s.Path != "" {
		return s.Path
	}
	return s.Name
}

func (a *App) connect(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	noFocus := fs.Bool("no-focus", false, "")
	cfgPath := fs.String("config", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("connect requires target")
	}
	target := fs.Arg(0)
	cfg, err := a.loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	sessions, err := a.collectAllowUnavailableHerdr(ctx, cfg, target)
	if err != nil {
		return err
	}
	currentWorkspaceID := os.Getenv("HERDR_WORKSPACE_ID")
	res, err := connectpkg.Connect(ctx, herdr.NewCLIClient(), sessions, target, connectpkg.Options{
		NoFocus: *noFocus,
		Namer:   func(ctx context.Context, p string) string { return namer.Namer{}.Name(ctx, p, cfg.DirLength) },
	})
	if err != nil {
		return err
	}
	if !*noFocus {
		a.recordWorkspaceSwitch(currentWorkspaceID, res.Session.WorkspaceID)
	}
	_, err = fmt.Fprintf(a.Out, "%s\n", res.Session.Name)
	return err
}

func (a *App) preview(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	cfgPath := fs.String("config", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("preview requires target")
	}
	target := fs.Arg(0)
	cfg, err := a.loadConfig(*cfgPath)
	if err != nil {
		return err
	}
	sessions, err := a.collectAllowUnavailableHerdr(ctx, cfg, target)
	if err != nil {
		return err
	}
	s, ok := connectpkg.Resolve(sessions, target)
	if !ok {
		s = model.Session{Name: filepath.Base(target), Path: target}
	}
	out, err := preview.Render(ctx, s, cfg.DefaultSessionConfig.PreviewCommand)
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(a.Out, out)
	return err
}

func (a *App) clone(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("clone", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	cmdDir := fs.String("cmdDir", "", "")
	dir := fs.String("dir", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("clone requires repo")
	}
	dest, err := clonepkg.Clone(ctx, clonepkg.Request{Repo: fs.Arg(0), CmdDir: *cmdDir, Dir: *dir})
	if err != nil {
		return err
	}
	return a.connect(ctx, []string{dest})
}

func (a *App) root(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("root", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	doConnect := fs.Bool("connect", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, err := gitRoot(ctx, ".")
	if err != nil {
		return err
	}
	if *doConnect {
		return a.connect(ctx, []string{root})
	}
	_, err = fmt.Fprintln(a.Out, root)
	return err
}

func gitRoot(ctx context.Context, dir string) (string, error) {
	b, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func (a *App) last(ctx context.Context, _ []string) error {
	historyDir, err := historyStateDir()
	if err != nil {
		return err
	}
	currentWorkspaceID := os.Getenv("HERDR_WORKSPACE_ID")
	id, ok, err := lastWorkspace(historyDir, currentWorkspaceID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("no previous workspace recorded")
	}
	if err := herdr.NewCLIClient().WorkspaceFocus(ctx, id); err != nil {
		return err
	}
	if err := state.RecordSwitch(historyDir, currentWorkspaceID, id); err != nil {
		a.warnf("could not record workspace history: %v", err)
	}
	return nil
}

func lastWorkspace(stateDir, currentWorkspaceID string) (string, bool, error) {
	if currentWorkspaceID == "" {
		return state.Last(stateDir)
	}
	return state.Previous(stateDir, currentWorkspaceID)
}

func (a *App) recordWorkspaceSwitch(fromWorkspaceID, toWorkspaceID string) {
	if fromWorkspaceID == "" || toWorkspaceID == "" {
		return
	}
	historyDir, err := historyStateDir()
	if err == nil {
		err = state.RecordSwitch(historyDir, fromWorkspaceID, toWorkspaceID)
	}
	if err != nil {
		a.warnf("could not record workspace history: %v", err)
	}
}

func historyStateDir() (string, error) {
	return state.SessionHistoryDir(os.Getenv("HERDR_PLUGIN_STATE_DIR"), os.Getenv("HERDR_SOCKET_PATH"))
}

func (a *App) plugin(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("unknown plugin command")
	}
	switch args[0] {
	case "open-picker":
		return herdr.NewCLIClient().PluginPaneOpen(ctx, "adriankarlen.yeet", "picker", "overlay")
	case "watch-history":
		return a.watchHistory(ctx)
	default:
		return errors.New("unknown plugin command")
	}
}

func (a *App) watchHistory(ctx context.Context) (err error) {
	stateDir := os.Getenv("HERDR_PLUGIN_STATE_DIR")
	socketPath := os.Getenv("HERDR_SOCKET_PATH")
	release, acquired, err := state.TryHistoryWatcherLock(stateDir, socketPath)
	if err != nil {
		return err
	}
	if acquired {
		defer func() { err = errors.Join(err, release()) }()
	}
	if !acquired && os.Getenv("HERDR_PLUGIN_EVENT") != "workspace.closed" {
		return nil
	}
	historyDir, err := state.SessionHistoryDir(stateDir, socketPath)
	if err != nil {
		return err
	}
	if err := applyHistoryHook(historyDir); err != nil {
		return err
	}
	if !acquired {
		return nil
	}

	return herdr.WatchWorkspaceEvents(ctx, socketPath,
		func(workspaceID string) error {
			return state.Record(historyDir, workspaceID)
		},
		func(workspaceID string) error {
			return state.RemoveWorkspace(historyDir, workspaceID)
		},
	)
}

func applyHistoryHook(historyDir string) error {
	eventName := os.Getenv("HERDR_PLUGIN_EVENT")
	if eventName == "" || eventName == "startup" {
		return nil
	}
	switch eventName {
	case "workspace.focused":
		return nil
	case "workspace.closed":
		var payload struct {
			Data struct {
				WorkspaceID string `json:"workspace_id"`
			} `json:"data"`
		}
		raw := os.Getenv("HERDR_PLUGIN_EVENT_JSON")
		if raw == "" {
			return errors.New("HERDR_PLUGIN_EVENT_JSON is required for workspace.closed")
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return fmt.Errorf("decode HERDR_PLUGIN_EVENT_JSON: %w", err)
		}
		if payload.Data.WorkspaceID == "" {
			return errors.New("workspace.closed event payload is missing data.workspace_id")
		}
		return state.RemoveWorkspace(historyDir, payload.Data.WorkspaceID)
	default:
		return fmt.Errorf("unknown Herdr plugin event %q", eventName)
	}
}
