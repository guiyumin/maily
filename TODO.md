# TODO

## 1. Support Gmail Labels

- [ ] List all Gmail labels via IMAP (exposed as mailboxes)
- [ ] Display labels in the UI
- [ ] Allow filtering/viewing emails by label
- [ ] Support adding/removing labels from emails

## 2. Support Gmail Directories

- [ ] Support special Gmail folders:
  - `[Gmail]/Sent Mail`
  - `[Gmail]/Spam`
  - `[Gmail]/Trash`
  - `[Gmail]/Drafts`
  - `[Gmail]/All Mail`
  - `[Gmail]/Starred`
- [ ] Add folder navigation in the UI
- [ ] Display current folder in the interface

## 3. Multiple Select and Bulk Actions

- [ ] Add multi-select mode (e.g., Shift+Space or visual mode)
- [ ] Support bulk delete for selected emails
- [ ] Support bulk mark as read/unread for selected emails
- [ ] Show selection count in UI

## 4. AI Summarization

- [ ] Integrate AI model for email summarization
- [ ] Add summarize command/shortcut for selected email
- [ ] Display summary in preview pane or modal
- [ ] Consider batch summarization for email threads

## 5. Support OAuth Login (Future)

For when the project has more traction. Unverified OAuth apps show scary warnings to users.

- [ ] Set up Google Cloud OAuth credentials
- [ ] Implement OAuth 2.0 flow with browser redirect
- [ ] Handle token storage and refresh
- [ ] Add `maily login gmail --oauth` option
- [ ] Consider Google app verification for wider distribution
