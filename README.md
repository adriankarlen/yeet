# yeet

`yeet` is a fast, minimal workspace picker plugin for [Herdr](https://github.com/herdrdev/herdr). It is inspired by [Sesh](https://github.com/joshmedeski/sesh).

> [!NOTE]
> I built this plugin for myself, using AI to write most of the code ("vibe coded"). I do not plan to maintain it actively. Feel free to use it or fork it, but expect no regular updates, support, or roadmap.

<img width="2314" height="1413" alt="image" src="https://github.com/user-attachments/assets/74ff3966-6759-4f5c-b223-b0f77a4a46e1" />

## What it does

Most session pickers show many columns and animate every keystroke. `yeet` skips that. It shows one line per workspace and reacts instantly.

- **Minimal interface.** `yeet` searches with [`gum filter`](https://github.com/charmbracelet/gum) by default, or with `fzf` if you choose it.
- **Your theme, not a new one.** `yeet` reads your existing `GUM_*` color variables. It does not add its own color palette.
- **No animation.** The picker has no cursor smear, no trails, and no extra render loop.
- **Optional preview.** With `fzf`, press `Ctrl+O` to toggle a side-by-side directory preview. It stays off until you ask for it.
- **Combines four sources.** `yeet` merges active Herdr workspaces, sessions from your config file, recent directories from `zoxide`, and (when you connect directly) the exact path you type.

## Install

### 1. Install the plugin through Herdr

```bash
herdr plugin install adriankarlen/yeet
```

To work on the plugin locally instead, link your checkout:

```bash
herdr plugin link .
```

### 2. Add keybindings

Add this to `~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "prefix+t"
type = "plugin_action"
command = "adriankarlen.yeet.open-picker"
description = "open Yeet picker"

[[keys.command]]
key = "prefix+shift+b"
type = "plugin_action"
command = "adriankarlen.yeet.last"
description = "switch to previous workspace"
```

## Commands

Run these from a terminal, or bind them as Herdr actions.

| Command | What it does |
| --- | --- |
| `yeet picker` | Opens the fuzzy picker. Uses `gum` unless you pass `-fzf` or set `HERDR_YEET_PICKER=fzf`. |
| `yeet picker -fzf` | Opens the picker with `fzf`. Press `Ctrl+O` for a side-by-side preview. |
| `yeet list` | Prints every discovered workspace as `source<TAB>name<TAB>path`. |
| `yeet list --icons` | Prints the same list with Nerd Font icons, matching `sesh list -i`. |
| `yeet list --json` | Prints the same list as JSON. |
| `yeet connect <target>` | Creates or focuses a workspace for `<target>` (a name or a path). Add `--no-focus` to connect without switching focus. |
| `yeet preview <target>` | Runs the configured preview command against `<target>` and prints the result. |
| `yeet last` | Switches back to the workspace you were on before the current one. |
| `yeet clone <repo>` | Runs `git clone <repo>`, then connects to the cloned directory. Add `--dir <name>` to choose the folder name, or `--cmdDir <path>` to clone under a specific parent directory. |
| `yeet root` | Prints the current Git repository's top-level directory. Add `--connect` to connect to it instead of printing it. |
| `yeet --version` | Prints the installed version. |

Every command that reads workspaces (`picker`, `list`, `connect`, `preview`) accepts `--config <path>` to use one config file instead of the normal search order below.

## Configuration

`yeet` looks for a config file in this order and stops at the first one it finds:

1. The path you pass with `--config`.
2. The path in the `HERDR_YEET_CONFIG` environment variable.
3. `config.toml` or `sesh.toml` inside the plugin's Herdr-managed config directory. Find that directory with `herdr plugin config-dir adriankarlen.yeet`.
4. `~/.config/yeet/config.toml` or `~/.config/yeet/sesh.toml`.
5. `~/.config/sesh/sesh.toml`, so an existing [Sesh](https://github.com/joshmedeski/sesh) config keeps working without changes.

If none of these exist, `yeet` runs with built-in defaults and no configured workspaces.

### Two file formats, chosen by name

The filename tells `yeet` which format to expect:

- **`config.toml`** uses the native schema described below. It must start with `version = 1`.
- **`sesh.toml`** uses the older, Sesh-compatible schema (`[[session]]`, `[[wildcard]]`, `[default_session]`, and so on). Keep using this format if you already have a Sesh config you want to reuse as is.

Naming a native-schema file `config.toml` and a legacy-schema file `sesh.toml` avoids parse errors. The two schemas use different keys, and `yeet` checks the file strictly against the format its name implies.

### Native config example (`config.toml`)

```toml
version = 1

[list]
# Sources to check, in order. Any of: "herdr", "config", "zoxide", "dir".
source_order = ["herdr", "config", "zoxide", "dir"]
blacklist = ["^scratch$"]

[naming]
path_components = 1

[picker]
show_icons = true
prompt = "> "
placeholder = "Search workspaces"
workspace_sort = "recent" # "workspace", "recent", or "agent"

[workspace_defaults]
startup = "git status"
preview = "eza --icons=always -la {}"

[[tab]]
name = "git"
startup = "git status"

[[workspace]]
name = "yeet"
path = "~/repos/yeet"
startup = "git status"
tabs = ["git"]

[[rule]]
path_glob = "~/repos/**"
preview = "eza --icons=always -la {}"
tabs = ["git"]
```

Every `[picker]`, `[naming]`, and `[list]` key is optional; omitted keys fall back to sensible defaults. `[[workspace]]` entries name one folder each. `[[rule]]` entries match many folders with a glob. `[[tab]]` entries are named startup scripts that a workspace or rule can reference by name.

This example does not show every key. Options such as `picker.show_path`, `picker.preview_mode`, `picker.herdr_theme_inherit`, `workspace.disable_startup`, and `keys.cycle_preview_mode` (the key that toggles the `fzf` preview, `ctrl+o` by default) also exist. This README does not keep a full field-by-field reference. Read `internal/config/native.go` in this repository for the complete, current list.

### Environment variables

| Variable | Effect |
| --- | --- |
| `HERDR_YEET_CONFIG` | Overrides the config file search above with one explicit path. |
| `HERDR_YEET_PICKER=fzf` | Makes `yeet picker` use `fzf` even without the `-fzf` flag. |

Herdr also injects its own variables (`HERDR_WORKSPACE_ID`, `HERDR_PLUGIN_STATE_DIR`, `HERDR_SOCKET_PATH`, and others) when it runs plugin commands. You do not need to set these yourself.
