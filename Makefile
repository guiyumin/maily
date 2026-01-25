.PHONY: build clean push cli patch minor major lint desktop

APP_NAME := maily
BUILD_DIR := build
VERSION_FILE := internal/version/version.go

# Get version from version.go (source of truth for CLI)
VERSION := $(shell grep 'Version = ' $(VERSION_FILE) | sed 's/.*"\(.*\)"/\1/')
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILD_TS := $(shell date +%s)

# Current version from version.go
CURRENT_VERSION := $(VERSION)

# For dev builds, append commit info and timestamp if dirty (ensures server restart on rebuild)
GIT_STATE := $(shell git diff --quiet 2>/dev/null || echo "-dirty")
BUILD_VERSION := $(VERSION)$(if $(GIT_STATE),+$(COMMIT)$(GIT_STATE)-$(BUILD_TS),)

LDFLAGS := -s -w \
	-X maily/internal/version.Version=$(BUILD_VERSION) \
	-X maily/internal/version.Commit=$(COMMIT) \
	-X maily/internal/version.Date=$(DATE)

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) .

clean:
	rm -rf $(BUILD_DIR)

push:
	git push origin main --tags

# CLI version bump: make cli <patch|minor|major>
cli:
	@if [ -z "$(filter patch minor major,$(MAKECMDGOALS))" ]; then \
		echo "Current CLI version: $(CURRENT_VERSION)"; \
		echo ""; \
		echo "Usage: make cli <patch|minor|major>"; \
		exit 1; \
	fi

lint:
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...

# Desktop version files
DESKTOP_TAURI_CONF := tauri/src-tauri/tauri.conf.json
DESKTOP_CARGO_TOML := tauri/src-tauri/Cargo.toml
DESKTOP_CARGO_LOCK := tauri/src-tauri/Cargo.lock
DESKTOP_PACKAGE_JSON := tauri/package.json
DESKTOP_VERSION := $(shell jq -r '.version' $(DESKTOP_TAURI_CONF) 2>/dev/null || echo "0.0.0")

# Desktop version bump: make desktop <patch|minor|major>
desktop:
	@if [ -z "$(filter patch minor major,$(MAKECMDGOALS))" ]; then \
		echo "Current desktop version: $(DESKTOP_VERSION)"; \
		echo ""; \
		echo "Usage: make desktop <patch|minor|major>"; \
		exit 1; \
	fi

# Unified patch/minor/major targets - work with both 'cli' and 'desktop'
patch minor major:
	@TYPE=$@ && \
	if echo "$(MAKECMDGOALS)" | grep -q "desktop"; then \
		CURRENT=$(DESKTOP_VERSION) && \
		echo "Current desktop version: $$CURRENT" && \
		IFS='.' read -r MAJOR MINOR PATCH <<< "$$CURRENT" && \
		if [ "$$TYPE" = "patch" ]; then PATCH=$$((PATCH + 1)); \
		elif [ "$$TYPE" = "minor" ]; then MINOR=$$((MINOR + 1)); PATCH=0; \
		elif [ "$$TYPE" = "major" ]; then MAJOR=$$((MAJOR + 1)); MINOR=0; PATCH=0; \
		fi && \
		NEW="$$MAJOR.$$MINOR.$$PATCH" && \
		echo "New desktop version: $$NEW" && \
		echo "" && \
		echo "Updating tauri/src-tauri/tauri.conf.json..." && \
		jq --arg v "$$NEW" '.version = $$v' $(DESKTOP_TAURI_CONF) > tmp.json && mv tmp.json $(DESKTOP_TAURI_CONF) && \
		echo "Updating tauri/package.json..." && \
		jq --arg v "$$NEW" '.version = $$v' $(DESKTOP_PACKAGE_JSON) > tmp.json && mv tmp.json $(DESKTOP_PACKAGE_JSON) && \
		echo "Updating tauri/src-tauri/Cargo.toml..." && \
		sed -i '' 's/^version = ".*"/version = "'$$NEW'"/' $(DESKTOP_CARGO_TOML) && \
		echo "Syncing Cargo.lock..." && \
		cargo generate-lockfile --manifest-path $(DESKTOP_CARGO_TOML) && \
		echo "" && \
		echo "Committing changes..." && \
		git add $(DESKTOP_TAURI_CONF) $(DESKTOP_PACKAGE_JSON) $(DESKTOP_CARGO_TOML) $(DESKTOP_CARGO_LOCK) && \
		git commit -m "chore(desktop): bump version to v$$NEW" && \
		echo "" && \
		echo "Creating tag desktop-v$$NEW..." && \
		git tag "desktop-v$$NEW" && \
		echo "" && \
		echo "✓ Desktop version $$NEW committed and tagged" && \
		echo "" && \
		echo "Run 'git push origin main --tags' to trigger release"; \
	else \
		echo "Current version: $(CURRENT_VERSION)" && \
		NEW_VERSION=$$(echo "$(CURRENT_VERSION)" | awk -F. -v type="$$TYPE" '{ \
			if (type == "major") { print $$1+1".0.0" } \
			else if (type == "minor") { print $$1"."$$2+1".0" } \
			else { print $$1"."$$2"."$$3+1 } \
		}') && \
		BUILD_DATE=$$(date -u +"%Y-%m-%d") && \
		echo "New version: $$NEW_VERSION" && \
		echo "Build date: $$BUILD_DATE" && \
		sed -i '' 's/Version = ".*"/Version = "'$$NEW_VERSION'"/' $(VERSION_FILE) && \
		sed -i '' 's/Date    = ".*"/Date    = "'$$BUILD_DATE'"/' $(VERSION_FILE) && \
		echo "" && \
		echo "Committing changes..." && \
		git add $(VERSION_FILE) && \
		git commit -m "chore: bump version to v$$NEW_VERSION" && \
		echo "" && \
		echo "Creating tag v$$NEW_VERSION..." && \
		git tag "v$$NEW_VERSION" && \
		echo "" && \
		echo "✓ CLI version $$NEW_VERSION committed and tagged" && \
		echo "" && \
		echo "Run 'git push origin main --tags' to trigger release"; \
	fi
