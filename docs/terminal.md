# Embedded Terminal for Maily Desktop

Analysis of options for embedding a terminal in the Maily Tauri desktop app.

## Requirements

From `desktop.md`:
- Terminal pane that can be toggled (like VSCode)
- Run Maily CLI commands directly
- Pipe email content to shell commands
- Output from commands can reference emails/events

## Option 1: libghostty (Recommended for Future)

**Ghostty** is a fast, native terminal emulator by Mitchell Hashimoto. The key asset is `libghostty` - a C-compatible library for embedding terminals.

### Architecture

```
Ghostty Source Structure:
src/
├── terminal/       # VT100 emulation, escape sequences
├── renderer/       # GPU rendering (Metal/OpenGL)
├── font/           # Font discovery, shaping
├── input/          # Keyboard/mouse handling
├── Surface.zig     # Main terminal surface
└── lib_vt.zig      # libghostty-vt entry point
```

### libghostty-vt

The first modular piece being extracted:

```
Purpose:     Terminal sequence parsing + state management
Platforms:   macOS, Linux, Windows, WebAssembly
Languages:   Zig, C (via C API)
Status:      Available, API not yet stable
```

Build commands:
```bash
zig build lib-vt                              # Native library
zig build lib-vt -Dtarget=wasm32-freestanding # WebAssembly
```

### Why libghostty for Maily?

| Aspect | Benefit |
|--------|---------|
| **Performance** | Metal/OpenGL rendering, 60fps under heavy load |
| **Standards** | Most compliant terminal emulator available |
| **Native** | No Electron overhead, matches Tauri philosophy |
| **Proven** | Ghostty macOS app is itself a libghostty consumer |
| **Active** | Mitchell Hashimoto actively developing |

### Integration Path

1. **Rust FFI** - Call libghostty C API from Tauri's Rust backend
2. **Surface embedding** - Render terminal surface in a native view
3. **Event bridging** - Forward keyboard/mouse from webview to terminal

```rust
// Conceptual Tauri integration
use libghostty::Terminal;

#[tauri::command]
fn create_terminal(app: AppHandle) -> Result<TerminalId, Error> {
    let terminal = Terminal::new(config)?;
    terminal.spawn_shell("/bin/zsh")?;
    // Embed in native window alongside webview
    Ok(terminal.id())
}

#[tauri::command]
fn terminal_write(id: TerminalId, data: &str) -> Result<(), Error> {
    terminals.get(id)?.write(data)
}
```

### Challenges

1. **API Stability** - libghostty API still evolving
2. **Zig Build** - Need to integrate Zig build into Tauri/Cargo
3. **Rendering** - Need to handle GPU context sharing with webview
4. **Platform Parity** - Metal (macOS), OpenGL (Linux) different code paths

### Roadmap Alignment

From Ghostty's roadmap:
```
| # | Step                                              | Status |
|---|---------------------------------------------------|--------|
| 6 | Cross-platform libghostty for Embeddable Terminals |   ⚠️   |
```

This is actively being developed. Track: https://github.com/ghostty-org/ghostty

## Option 2: xterm.js + portable-pty (Recommended for MVP)

Web-based terminal, commonly used in Electron apps (VSCode, Hyper).

```
xterm.js      → Terminal UI rendering in canvas/webgl
portable-pty  → PTY spawning (Rust crate, works with Tauri)
```

### Integration

```typescript
// Frontend (webview)
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';

const term = new Terminal();
term.loadAddon(new FitAddon());
term.open(document.getElementById('terminal'));

// Communicate with Tauri backend for PTY
```

```rust
// Tauri backend - use portable-pty crate
use portable_pty::{native_pty_system, PtySize, CommandBuilder};

#[tauri::command]
fn spawn_pty() -> Result<PtyHandle, Error> {
    let pty_system = native_pty_system();
    let pair = pty_system.openpty(PtySize { rows: 24, cols: 80, .. })?;
    let cmd = CommandBuilder::new("zsh");
    pair.slave.spawn_command(cmd)?;
    Ok(handle)
}
```

### Pros/Cons

| Pros | Cons |
|------|------|
| Mature ecosystem | WebGL rendering (not native Metal) |
| Works in webview | Slightly heavier than native |
| VSCode-proven | Canvas-based, not GPU-native |
| Easier initial setup | |

### Tauri Compatibility

- **xterm.js**: Works in Tauri webview
- **portable-pty**: Rust crate, works natively with Tauri

## Option 3: Alacritty Components

Alacritty is a fast, Rust-based terminal. Could potentially extract components.

```
Language:    Rust (native to Tauri)
Rendering:   OpenGL (crossplatform), no Metal
Status:      Not designed as library
```

### Reality Check

Alacritty isn't designed for embedding. Would require significant fork effort. Not recommended unless other options fail.

## Option 4: WezTerm

WezTerm by Wez Furlong, also Rust-based.

```
Language:    Rust
Rendering:   WebGPU (modern), OpenGL fallback
Features:    Multiplexer built-in, Lua scripting
```

WezTerm has better architecture for embedding than Alacritty, but still not officially a library.

## Recommendation

### Phase 1: xterm.js + portable-pty (MVP)

Start here for fastest time-to-working:

```
Frontend:  xterm.js in Tauri webview
Backend:   portable-pty crate for PTY handling
IPC:       Tauri commands for bidirectional communication
```

This gets a working terminal quickly. Performance is good enough for CLI usage.

### Phase 2: libghostty Migration (Future)

When libghostty stabilizes (watch for stable release):

1. Replace xterm.js canvas with native libghostty surface
2. Keep PTY handling in Rust
3. Gain Metal rendering on macOS, better performance

## Implementation Sketch (Phase 1)

```
maily-desktop/
├── src-tauri/
│   ├── src/
│   │   ├── terminal/
│   │   │   ├── mod.rs        # PTY management
│   │   │   ├── pty.rs        # portable-pty wrapper
│   │   │   └── commands.rs   # Tauri commands
│   │   └── main.rs
│   └── Cargo.toml            # Add portable-pty
└── src/
    └── components/
        └── Terminal.tsx       # xterm.js component
```

### Cargo.toml

```toml
[dependencies]
portable-pty = "0.8"
```

### PTY Manager (Rust)

```rust
// src-tauri/src/terminal/mod.rs
use portable_pty::{native_pty_system, CommandBuilder, PtySize};
use std::collections::HashMap;
use std::sync::Mutex;
use tauri::{AppHandle, Manager};

pub struct TerminalManager {
    terminals: Mutex<HashMap<u32, TerminalInstance>>,
    next_id: Mutex<u32>,
}

impl TerminalManager {
    pub fn new() -> Self {
        Self {
            terminals: Mutex::new(HashMap::new()),
            next_id: Mutex::new(0),
        }
    }

    pub fn spawn(&self, app: &AppHandle) -> Result<u32, String> {
        let pty_system = native_pty_system();
        let pair = pty_system
            .openpty(PtySize {
                rows: 24,
                cols: 80,
                pixel_width: 0,
                pixel_height: 0,
            })
            .map_err(|e| e.to_string())?;

        let cmd = CommandBuilder::new(std::env::var("SHELL").unwrap_or("/bin/zsh".into()));
        let child = pair.slave.spawn_command(cmd).map_err(|e| e.to_string())?;

        let mut id = self.next_id.lock().unwrap();
        let terminal_id = *id;
        *id += 1;

        // Spawn reader thread
        let reader = pair.master.try_clone_reader().unwrap();
        let app_clone = app.clone();
        std::thread::spawn(move || {
            use std::io::Read;
            let mut reader = reader;
            let mut buf = [0u8; 4096];
            loop {
                match reader.read(&mut buf) {
                    Ok(0) => break,
                    Ok(n) => {
                        let data = String::from_utf8_lossy(&buf[..n]).to_string();
                        app_clone.emit_all(&format!("terminal-output-{}", terminal_id), data).ok();
                    }
                    Err(_) => break,
                }
            }
        });

        self.terminals.lock().unwrap().insert(
            terminal_id,
            TerminalInstance {
                master: pair.master,
                child,
            },
        );

        Ok(terminal_id)
    }

    pub fn write(&self, id: u32, data: &str) -> Result<(), String> {
        use std::io::Write;
        let mut terminals = self.terminals.lock().unwrap();
        let terminal = terminals.get_mut(&id).ok_or("Terminal not found")?;
        terminal
            .master
            .write_all(data.as_bytes())
            .map_err(|e| e.to_string())
    }

    pub fn resize(&self, id: u32, rows: u16, cols: u16) -> Result<(), String> {
        let terminals = self.terminals.lock().unwrap();
        let terminal = terminals.get(&id).ok_or("Terminal not found")?;
        terminal
            .master
            .resize(PtySize {
                rows,
                cols,
                pixel_width: 0,
                pixel_height: 0,
            })
            .map_err(|e| e.to_string())
    }
}

struct TerminalInstance {
    master: Box<dyn portable_pty::MasterPty + Send>,
    child: Box<dyn portable_pty::Child + Send>,
}
```

### Tauri Commands

```rust
// src-tauri/src/terminal/commands.rs
use super::TerminalManager;
use tauri::State;

#[tauri::command]
pub fn spawn_terminal(
    app: tauri::AppHandle,
    manager: State<TerminalManager>,
) -> Result<u32, String> {
    manager.spawn(&app)
}

#[tauri::command]
pub fn terminal_input(
    id: u32,
    data: String,
    manager: State<TerminalManager>,
) -> Result<(), String> {
    manager.write(id, &data)
}

#[tauri::command]
pub fn terminal_resize(
    id: u32,
    rows: u16,
    cols: u16,
    manager: State<TerminalManager>,
) -> Result<(), String> {
    manager.resize(id, rows, cols)
}
```

### Terminal Component (Frontend)

```typescript
// src/components/Terminal.tsx
import { useEffect, useRef } from 'react';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { WebglAddon } from 'xterm-addon-webgl';
import { invoke } from '@tauri-apps/api/tauri';
import { listen } from '@tauri-apps/api/event';

interface TerminalPaneProps {
  visible: boolean;
}

export function TerminalPane({ visible }: TerminalPaneProps) {
  const termRef = useRef<HTMLDivElement>(null);
  const termInstance = useRef<Terminal | null>(null);
  const fitAddon = useRef<FitAddon | null>(null);
  const terminalId = useRef<number | null>(null);

  useEffect(() => {
    if (!termRef.current || termInstance.current) return;

    const term = new Terminal({
      fontSize: 14,
      fontFamily: 'JetBrains Mono, Menlo, monospace',
      theme: {
        background: '#1a1a1a',
        foreground: '#e0e0e0',
      },
    });

    const fit = new FitAddon();
    term.loadAddon(fit);

    try {
      term.loadAddon(new WebglAddon());
    } catch (e) {
      console.warn('WebGL addon failed, using canvas renderer');
    }

    term.open(termRef.current);
    fit.fit();

    termInstance.current = term;
    fitAddon.current = fit;

    // Spawn PTY
    invoke<number>('spawn_terminal').then((id) => {
      terminalId.current = id;

      // Listen for output
      listen<string>(`terminal-output-${id}`, (event) => {
        term.write(event.payload);
      });

      // Send input
      term.onData((data) => {
        invoke('terminal_input', { id, data });
      });

      // Handle resize
      term.onResize(({ rows, cols }) => {
        invoke('terminal_resize', { id, rows, cols });
      });
    });

    // Resize observer
    const resizeObserver = new ResizeObserver(() => {
      fit.fit();
    });
    resizeObserver.observe(termRef.current);

    return () => {
      resizeObserver.disconnect();
      term.dispose();
    };
  }, []);

  // Re-fit when visibility changes
  useEffect(() => {
    if (visible && fitAddon.current) {
      setTimeout(() => fitAddon.current?.fit(), 0);
    }
  }, [visible]);

  return (
    <div
      ref={termRef}
      className={`h-full w-full ${visible ? '' : 'hidden'}`}
      style={{ padding: '8px', backgroundColor: '#1a1a1a' }}
    />
  );
}
```

### Package.json Dependencies

```json
{
  "dependencies": {
    "xterm": "^5.3.0",
    "xterm-addon-fit": "^0.8.0",
    "xterm-addon-webgl": "^0.16.0"
  }
}
```

## Maily-Specific Features

Once basic terminal works, add Maily integrations:

### 1. Email Piping

```bash
# Pipe email to command
maily read <id> | grep "error"

# Attach command output to email
echo "Deploy log" | maily compose --to ops@company.com
```

### 2. Context Awareness

Terminal knows current email context:
```bash
# $MAILY_CURRENT_EMAIL set when reading an email
maily reply --stdin < response.txt
```

### 3. Quick Commands

Built-in shortcuts:
```
Ctrl+`        → Toggle terminal
Cmd+K t       → Focus terminal (from command palette)
```

### 4. Output Linking

Parse terminal output for email/event references:
```
[email:abc123] mentioned in output → clickable link
```

## Resources

- Ghostty: https://github.com/ghostty-org/ghostty
- libghostty blog post: https://mitchellh.com/writing/libghostty-is-coming
- xterm.js: https://xtermjs.org/
- portable-pty: https://crates.io/crates/portable-pty
- Tauri: https://tauri.app/

## Decision Log

| Date | Decision | Rationale |
|------|----------|-----------|
| 2025-01-11 | Start with xterm.js + portable-pty | Fastest to working, proven stack |
| Future | Plan libghostty migration | Better performance, native Metal rendering |
