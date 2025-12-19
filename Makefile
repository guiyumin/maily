.PHONY: build clean

APP_NAME := cocomail
BUILD_DIR := build

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) .


clean:
	rm -rf $(BUILD_DIR)


.PHONY: push
push:
	git push origin main --tags
