.PHONY: dev build version patch minor major

TAURI_CONF := src-tauri/tauri.conf.json
CURRENT_VERSION := $(shell jq -r '.version' $(TAURI_CONF))

dev:
	bun run tauri dev

build:
	bun run tauri build

# Version bump: make version patch|minor|major
version:
	@if [ -z "$(filter patch minor major,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make version <patch|minor|major>"; \
		echo "Current version: $(CURRENT_VERSION)"; \
		exit 1; \
	fi

patch minor major: version
	@TYPE=$@ && \
	CURRENT=$(CURRENT_VERSION) && \
	echo "Current version: $$CURRENT" && \
	IFS='.' read -r MAJOR MINOR PATCH <<< "$$CURRENT" && \
	if [ "$$TYPE" = "patch" ]; then PATCH=$$((PATCH + 1)); \
	elif [ "$$TYPE" = "minor" ]; then MINOR=$$((MINOR + 1)); PATCH=0; \
	elif [ "$$TYPE" = "major" ]; then MAJOR=$$((MAJOR + 1)); MINOR=0; PATCH=0; \
	fi && \
	NEW="$$MAJOR.$$MINOR.$$PATCH" && \
	echo "New version: $$NEW" && \
	jq --arg v "$$NEW" '.version = $$v' $(TAURI_CONF) > tmp.json && mv tmp.json $(TAURI_CONF) && \
	git add $(TAURI_CONF) && \
	git commit -m "chore(desktop): bump version to v$$NEW" && \
	git tag "desktop-v$$NEW" && \
	echo "Created tag desktop-v$$NEW" && \
	echo "Run 'git push origin main --tags' to push changes"
