.PHONY: build clean push version patch minor major lint

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

# Version bump: make version <patch|minor|major>
version:
	@if [ -z "$(filter patch minor major,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make version <patch|minor|major>"; \
		echo "Current version: $(CURRENT_VERSION)"; \
		exit 1; \
	fi

patch minor major: version
	@TYPE=$@ && \
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
	git add $(VERSION_FILE) && \
	git commit -m "chore: bump version to v$$NEW_VERSION" && \
	git tag "v$$NEW_VERSION" && \
	echo "Created tag v$$NEW_VERSION" && \
	echo "Run 'make push' to push changes and trigger release"

lint:
	go run honnef.co/go/tools/cmd/staticcheck@latest ./...
