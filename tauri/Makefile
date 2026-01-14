.PHONY: dev build version

dev:
	bun run tauri dev

build:
	bun run tauri build

# Version bump: make version patch|minor|major
version:
	@if [ -z "$(filter patch minor major,$(word 2,$(MAKECMDGOALS)))" ]; then \
		echo "Usage: make version [patch|minor|major]"; \
		exit 1; \
	fi
	@TYPE=$(word 2,$(MAKECMDGOALS)); \
	CURRENT=$$(jq -r '.version' src-tauri/tauri.conf.json); \
	IFS='.' read -r MAJOR MINOR PATCH <<< "$$CURRENT"; \
	if [ "$$TYPE" = "patch" ]; then PATCH=$$((PATCH + 1)); \
	elif [ "$$TYPE" = "minor" ]; then MINOR=$$((MINOR + 1)); PATCH=0; \
	elif [ "$$TYPE" = "major" ]; then MAJOR=$$((MAJOR + 1)); MINOR=0; PATCH=0; \
	fi; \
	NEW="$$MAJOR.$$MINOR.$$PATCH"; \
	jq --arg v "$$NEW" '.version = $$v' src-tauri/tauri.conf.json > tmp.json && mv tmp.json src-tauri/tauri.conf.json; \
	echo "Version bumped: $$CURRENT -> $$NEW"

patch minor major:
	@: