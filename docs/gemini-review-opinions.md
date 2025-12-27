# Project Review: Email Provider Support and Extensibility

## Overview

This document reviews the current implementation of email provider support (specifically Gmail and Yahoo) in the `maily` project. The goal is to assess the codebase's readiness for adding new providers and identify architectural improvements.

## Current Architecture Analysis

The current implementation supports Gmail and Yahoo Mail but does so through "hardcoded" logic and conditional checks scattered across multiple layers of the application. This approach makes adding new providers difficult and error-prone.

### Key Findings

#### 1. Hardcoded Provider Selection
**File:** `internal/ui/provider_selector.go`

The list of available providers is manually defined in a slice:
```go
var providers = []provider{
    {id: "gmail", name: "Gmail", desc: "Google Mail"},
    {id: "yahoo", name: "Yahoo Mail", desc: "Yahoo Mail"},
}
```
**Issue:** Adding a provider requires modifying the UI code directly. There is no dynamic loading of providers.

#### 2. Hardcoded Configuration and Credentials
**File:** `internal/auth/credentials.go`

Connection settings are hardcoded in helper functions:
- `GmailCredentials`: sets `imap.gmail.com` / `smtp.gmail.com`.
- `YahooCredentials`: sets `imap.mail.yahoo.com` / `smtp.mail.yahoo.com`.
- `PromptGmailCredentials`: This function is highly specific to Gmail's authentication flow (App Passwords) and prompt text.

**Issue:** This violates the Open-Closed Principle. To add a provider, you must write new helper functions and update the login flow to use them.

#### 3. Conditional Search Logic
**File:** `internal/mail/search.go`

The search functionality explicitly checks the host string to determine behavior:
```go
if strings.Contains(creds.IMAPHost, "gmail") {
    return GmailSearch(creds, mailbox, query)
}
return StandardSearch(creds, mailbox, query)
```
**Issue:** "Magic string" checks are brittle. If a custom Gmail domain (Google Workspace) is used that doesn't contain "gmail" in the host (unlikely for IMAP host, but possible in principle), this logic fails. More importantly, extending this requires adding `if/else` blocks for every provider with custom search needs.

Additionally, `GmailSearch` and `StandardSearch` contain significant code duplication for establishing connections and logging in.

#### 4. Hardcoded Folder Mapping
**File:** `internal/mail/imap.go`

Special folders like Trash, Archive, and Drafts are resolved using hardcoded checks for provider-specific paths:
```go
gmailTrash := "[Gmail]/Trash"
if c.mailboxExists(gmailTrash) { ... }
```
**Issue:** Each new provider with unique folder naming conventions will require new logic in `imap.go`.

## Recommendations

To support more email services in the future, I recommend refactoring toward a **Configuration-Driven** and **Interface-Based** architecture.

### 1. Define a Provider Interface
Create an interface that abstracts provider-specific behaviors:

```go
type Provider interface {
    ID() string
    Name() string
    IMAPHost() string
    SMTPHost() string
    GetTrashFolder() string
    GetSearchCapability() SearchCapability
    // ...
}
```

### 2. Implement a Provider Registry
Instead of hardcoded slices, use a registry pattern or a configuration file to load providers.

**Example `providers.yaml`:**
```yaml
providers:
  - id: gmail
    name: Gmail
    imap_host: imap.gmail.com
    special_folders:
      trash: "[Gmail]/Trash"
    search_extension: X-GM-RAW
  - id: yahoo
    name: Yahoo
    imap_host: imap.mail.yahoo.com
    # ...
```

### 3. Abstract Search Strategies
Refactor `internal/mail/search.go` to use a Strategy pattern. The `Search` function should delegate to a strategy based on the provider's configuration, not the hostname string.

- `SearchStrategy` interface.
- `GmailSearchStrategy` (implements `X-GM-RAW`).
- `StandardSearchStrategy` (implements `TEXT` search).

### 4. Unify Connection Logic
Refactor the search functions to reuse a shared connection/login helper method to reduce code duplication in `internal/mail/search.go`.

## Conclusion
The project currently works for the two targeted providers but will face scaling challenges. By adopting a configuration-driven approach and abstracting provider quirks behind interfaces, `maily` can easily support Outlook, iCloud, and custom IMAP servers in the future.
