import { useState, useCallback } from "react";
import { Terminal as TerminalIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";

/**
 * Terminal component - Placeholder for libghostty integration
 *
 * libghostty is a C-ABI compatible library from the Ghostty terminal emulator.
 * It provides terminal emulation, font handling, and rendering capabilities.
 *
 * Current status (Jan 2025):
 * - libghostty-vt is available but no stable release yet
 * - API is not yet finalized
 * - Works via C API that can be called from Rust
 *
 * Integration path when libghostty is stable:
 * 1. Add libghostty-vt to Cargo.toml as a dependency
 * 2. Create Rust bindings for the C API
 * 3. Create a Tauri command to spawn PTY and connect to libghostty
 * 4. Use WebGL/WebGPU canvas to render the terminal output
 * 5. Handle input events and send to PTY
 *
 * Resources:
 * - https://github.com/ghostty-org/ghostty
 * - https://mitchellh.com/writing/libghostty-is-coming
 */

interface TerminalProps {
  trigger?: React.ReactNode;
}

export function TerminalPlaceholder({ trigger }: TerminalProps) {
  const [open, setOpen] = useState(false);
  const [output, setOutput] = useState<string[]>([
    "Maily Terminal (Placeholder)",
    "libghostty integration coming soon...",
    "",
    "This component will integrate libghostty for:",
    "- Running maily CLI commands",
    "- Quick access to email operations",
    "- Account management",
    "",
    "For now, please use the system terminal:",
    "$ maily --help",
    "",
  ]);
  const [input, setInput] = useState("");

  const handleCommand = useCallback(() => {
    if (!input.trim()) return;

    setOutput((prev) => [...prev, `$ ${input}`, "Command execution not yet implemented", ""]);
    setInput("");
  }, [input]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleCommand();
    }
  };

  const defaultTrigger = (
    <Button variant="outline" size="icon">
      <TerminalIcon className="h-4 w-4" />
    </Button>
  );

  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>{trigger || defaultTrigger}</SheetTrigger>
      <SheetContent side="bottom" className="h-[400px] flex flex-col">
        <SheetHeader className="flex-shrink-0 flex flex-row items-center justify-between">
          <SheetTitle className="flex items-center gap-2">
            <TerminalIcon className="h-5 w-5" />
            Terminal
          </SheetTitle>
          <span className="text-xs text-muted-foreground bg-muted px-2 py-1 rounded">
            libghostty coming soon
          </span>
        </SheetHeader>

        <div className="flex-1 bg-zinc-950 rounded-lg p-4 font-mono text-sm overflow-hidden flex flex-col">
          {/* Output area */}
          <div className="flex-1 overflow-y-auto text-zinc-300">
            {output.map((line, i) => (
              <div key={i} className="leading-relaxed">
                {line || "\u00A0"}
              </div>
            ))}
          </div>

          {/* Input area */}
          <div className="flex items-center gap-2 mt-2 border-t border-zinc-800 pt-2">
            <span className="text-green-400">$</span>
            <input
              type="text"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Enter command..."
              className="flex-1 bg-transparent text-zinc-300 outline-none"
              autoFocus
            />
          </div>
        </div>
      </SheetContent>
    </Sheet>
  );
}

/**
 * Future implementation notes for libghostty:
 *
 * When libghostty becomes stable, the integration would look like:
 *
 * Rust side (src-tauri/src/terminal.rs):
 * ```rust
 * use libghostty_vt::{Terminal, Parser};
 * use std::process::{Command, Stdio};
 * use std::os::unix::process::CommandExt;
 * use nix::pty::{openpty, Winsize};
 *
 * pub struct TerminalState {
 *     terminal: Terminal,
 *     parser: Parser,
 *     pty_master: File,
 *     child: Child,
 * }
 *
 * #[tauri::command]
 * fn create_terminal(rows: u16, cols: u16) -> Result<TerminalId, String> {
 *     // Open PTY
 *     let winsize = Winsize { ws_row: rows, ws_col: cols, ws_xpixel: 0, ws_ypixel: 0 };
 *     let (master, slave) = openpty(&winsize)?;
 *
 *     // Spawn shell
 *     let child = Command::new(std::env::var("SHELL").unwrap_or("/bin/bash".into()))
 *         .stdin(Stdio::from(slave))
 *         .stdout(Stdio::from(slave))
 *         .stderr(Stdio::from(slave))
 *         .spawn()?;
 *
 *     // Create libghostty terminal
 *     let terminal = Terminal::new(rows, cols);
 *     let parser = Parser::new();
 *
 *     // Store and return ID
 *     Ok(store_terminal(TerminalState { terminal, parser, pty_master: master, child }))
 * }
 *
 * #[tauri::command]
 * fn write_terminal(id: TerminalId, data: String) -> Result<(), String> {
 *     let state = get_terminal(id)?;
 *     state.pty_master.write_all(data.as_bytes())?;
 *     Ok(())
 * }
 *
 * #[tauri::command]
 * fn read_terminal(id: TerminalId) -> Result<TerminalUpdate, String> {
 *     let state = get_terminal_mut(id)?;
 *     let mut buffer = [0u8; 4096];
 *     let n = state.pty_master.read(&mut buffer)?;
 *     state.parser.process(&buffer[..n], &mut state.terminal);
 *     Ok(state.terminal.get_updates())
 * }
 * ```
 *
 * Frontend side:
 * - Use WebGL canvas or raw HTML rendering
 * - libghostty can encode terminal content as HTML
 * - Handle keyboard input and send to backend
 * - Poll for updates and re-render
 */
