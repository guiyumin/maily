.PHONY: build clean push version patch minor major

APP_NAME := maily
BUILD_DIR := build
VERSION_FILE := internal/version/version.go

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Get current version from latest git tag (strips 'v' prefix)
CURRENT_VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.0.0")

LDFLAGS := -s -w \
	-X maily/internal/version.Version=$(VERSION) \
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
