# Maily Desktop

A handy email client app, and more.

## Features

- Multi-account email management (Gmail, Yahoo, IMAP)
- Calendar and Reminders integration (macOS)
- AI-powered features (summarization, event extraction)
- Keyboard-driven interface
- Email tagging and organization

## Requirements

- macOS 10.15+
- [Bun](https://bun.sh/) (package manager)
- Rust toolchain

## Development

```bash
# Install dependencies
bun install

# Run development server
make dev
```

## Building

```bash
# Build for release
make build
```

This creates:
- `src-tauri/target/release/bundle/dmg/` - macOS DMG installer
- `src-tauri/target/release/bundle/macos/` - macOS app bundle

## Version Management

```bash
make version patch   # 0.0.X
make version minor   # 0.X.0
make version major   # X.0.0
```

## Tech Stack

- **Frontend**: React 19, TypeScript, TanStack Router, Tailwind CSS, Radix UI
- **Backend**: Rust, Tauri 2, Tokio
- **State**: Zustand
- **Database**: SQLite (rusqlite)
- **Email**: IMAP/SMTP (imap, lettre)

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](../LICENSE) for details.
