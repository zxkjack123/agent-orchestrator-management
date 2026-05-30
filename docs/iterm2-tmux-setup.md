# Using AOM with iTerm2 + tmux Integration

iTerm2's native tmux integration (`tmux -CC`) gives each tmux window its own iTerm2 native window and each pane its own iTerm2 split — no keyboard shortcut gymnastics, full scrollback, and clipboard support everywhere. Combined with `aom orchestrate`, this is the most ergonomic way to watch a multi-agent team work in real time.

## How iTerm2 tmux Integration Works

When tmux is launched with the `-CC` flag, iTerm2 takes over as the frontend:

| tmux concept | iTerm2 appearance |
|---|---|
| tmux session | One iTerm2 window group |
| tmux window | One iTerm2 native window (⌘1, ⌘2, …) |
| tmux pane (split) | One iTerm2 pane split within the window |

AOM's team grid (`aom orchestrate`) places every agent into a single tmux window with multiple panes — so in iTerm2 you get **one native window showing all agents side by side**.

## Setup

### 1. Install tmux

```bash
brew install tmux
```

### 2. Configure iTerm2

In **iTerm2 → Settings → General → tmux**:

- Set **"When attaching, restore windows as"** → **Native windows**
- Optionally enable **"Automatically bury the tmux client session"** so the raw tmux client tab is hidden

### 3. Open AOM's tmux session via iTerm2

Instead of plain `tmux attach`, use:

```bash
tmux -CC attach -t <session-name>
```

Or let AOM create the session for you:

```bash
cd your-project
aom orchestrate --real
```

AOM detects that you are already inside tmux (`$TMUX` is set) and uses `switch-client` instead of `attach-session`, so you never get the nested-tmux error.

If you are **outside** any tmux session (e.g. a plain iTerm2 tab), `aom orchestrate` will call `attach-session` and iTerm2 will open native windows for each tmux window automatically.

## Recommended Workflow

```
iTerm2 native window ⌘1  →  "team" window (all agents in one grid)
                              ┌──────────────┬──────────────┐
                              │   builder    │   frontend   │
                              ├──────────────┼──────────────┤
                              │   reviewer   │  codex-be    │
                              └──────────────┴──────────────┘
```

### Step-by-step

1. Open a new iTerm2 tab (plain shell, no tmux yet).

2. Navigate to your project:

   ```bash
   cd ~/projects/my-project
   ```

3. Launch the full team grid:

   ```bash
   aom orchestrate --real
   ```

   AOM will:
   - Create (or reuse) a tmux session named after your project
   - Create a `team` window with one pane per agent
   - Apply `tiled` layout
   - Attach via iTerm2 — each tmux window becomes an iTerm2 native window

4. You land on the **team** window showing all agents in a grid. Use standard iTerm2 pane navigation (⌘⌥→ / ⌘⌥←) or Ctrl+B then arrow keys (tmux bindings).

5. To jump directly to one agent's pane from anywhere:

   ```bash
   aom switch builder     # focuses builder's pane in tmux + switches iTerm2 to that window
   ```

6. To return to the grid view:

   ```bash
   aom team view
   ```

## Pane Layout Options

```bash
aom orchestrate --layout tiled             # equal grid (default)
aom orchestrate --layout even-horizontal   # all agents in a row
aom orchestrate --layout even-vertical     # all agents in a column
aom orchestrate --layout main-vertical     # one large pane left, others stacked right
```

## Tips

### Increase scrollback

tmux default scrollback is 2000 lines. Agents produce a lot of output — increase it in `~/.tmux.conf`:

```
set -g history-limit 50000
```

### Mouse support

Add to `~/.tmux.conf` to scroll and click between panes:

```
set -g mouse on
```

### Pane borders with agent names

AOM sets each pane's title to the agent name via `select-pane -T`. To show titles in pane borders, add to `~/.tmux.conf`:

```
set -g pane-border-status top
set -g pane-border-format " #{pane_title} "
```

### Avoid window rename conflicts

AOM disables `automatic-rename` on the team window so tmux doesn't rename `team` to the running process name (e.g. `claude`). No manual config needed.

## Keyboard Reference

| Action | Keys |
|---|---|
| Switch pane (tmux) | Ctrl+B then arrow |
| Scroll up in pane | Ctrl+B then `[`, then arrows / PgUp |
| Exit scroll mode | `q` |
| Switch iTerm2 window | ⌘1 / ⌘2 / ⌘3 … |
| Switch iTerm2 pane | ⌘⌥ + arrow |
| Zoom a pane | Ctrl+B then `z` (toggle) |
| Detach from session | Ctrl+B then `d` |

## Troubleshooting

### "sessions should be nested with care" error

This happens if you run `aom team view` from inside a tmux pane that is already attached to the session. AOM handles this automatically by using `switch-client` instead of `attach-session` when `$TMUX` is set. If you still see the error, upgrade to the latest AOM binary.

### New iTerm2 windows appear for each agent

This is the expected iTerm2 -CC behaviour: each tmux **window** = one iTerm2 window. If you see 4 windows instead of 1, it means agents were spawned in separate solo windows before `aom orchestrate` ran. Re-run `aom orchestrate --real` — it will detect agents already in the team window and kill the stale solo windows automatically.

### Agent panes show blank / shell prompt instead of running agent

The agent process may have exited. Check:

```bash
aom session list --active   # shows only live sessions
aom doctor                  # checks auth, PATH, workspace setup
```

Then respawn:

```bash
aom orchestrate --real
```
