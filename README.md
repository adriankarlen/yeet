# yeet

An opinionated, fast, and minimal [Sesh](https://github.com/joshmedeski/sesh) workspace session manager plugin for [Herdr](https://github.com/herdrdev/herdr).

<img width="2314" height="1413" alt="image" src="https://github.com/user-attachments/assets/74ff3966-6759-4f5c-b223-b0f77a4a46e1" />


Unlike heavy multi-column session pickers, `yeet` is designed for speed:
- **Minimal interface**: Fast fuzzy search powered by [`gum filter`](https://github.com/charmbracelet/gum) or `fzf`.
- **Global styling**: Uses your environment theme (e.g. `GUM_*` colors such as Rose Pine) directly without forcing custom color palettes.
- **Zero animations**: No cursor smear, trails, or laggy UI loops.
- **Side-by-side preview**: Untoggled by default. When using `fzf`, toggle side-by-side directory preview instantly with `Ctrl+O`.
- **Sesh & Zoxide support**: Merges active Herdr workspaces, configured sessions (`~/.config/sesh/sesh.toml`), and recent directories from `zoxide`.

## Installation

### 1. Install via Herdr

```bash
herdr plugin install adriankarlen/yeet
```

Or link locally for development:

```bash
herdr plugin link .
```

### 2. Configure Keybindings

Add to your `~/.config/herdr/config.toml`:

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

## Features & Flags

- `yeet picker`: Opens the interactive fuzzy picker (uses `gum` by default, or `fzf` if specified/configured).
- `yeet picker -fzf`: Opens the picker using `fzf` with `Ctrl+O` toggleable side-by-side preview.
- `yeet last`: Instantly switches back to the last active workspace.
- `yeet list --icons`: Lists all discovered sessions with Nerd Font icons matching `sesh list -i`.
- `yeet connect <target>`: Creates or focuses a workspace directly.
