# Use bash for better shell syntax
SHELL := /bin/bash
TIMEFORMAT = 🕐 Total time: %R seconds

# Project Paths
APP_IMPORT=./cmd/import
APP_DOWNLOAD=./cmd/download
APP_QUERY=./cmd/query
DB_FILE=commons.db

# Session range for downloading votes
START_SESSION = 43-1
END_SESSION = 44-1

# Grab current Git commit short SHA
GIT_COMMIT := $(shell git rev-parse --short HEAD)

# Function to generate README.txt dynamically
define generate_readme
	echo "Commons Votes CLI Tools\n========================\n\nThis archive contains the statically linked CLI tools to download, import, and query Canadian Parliament voting data.\n\nPlatform: $(1)\nGit Commit: $(GIT_COMMIT)\n\nIncluded tools:\n- import_$(2)\n- download_$(2)\n- query_$(2)\n\nUsage:\n    ./import_$(2)\n    ./download_$(2)\n    ./query_$(2)\n\nNotes:\n- All binaries are statically linked. No installation needed.\n- Database file: commons.db (SQLite)\n- Downloads stored under ./downloads/\n" > bin/README.txt
endef

.PHONY: all
all: build

.PHONY: build
build:
	go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/import $(APP_IMPORT)
	go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/download $(APP_DOWNLOAD)
	go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/query $(APP_QUERY)

.PHONY: static-build
static-build: static-import static-download static-query

.PHONY: static-import
static-import:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/import_linux_amd64 $(APP_IMPORT)
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/import_linux_arm64 $(APP_IMPORT)
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/import_windows_amd64.exe $(APP_IMPORT)
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/import_mac_amd64 $(APP_IMPORT)
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/import_mac_arm64 $(APP_IMPORT)

.PHONY: static-download
static-download:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/download_linux_amd64 $(APP_DOWNLOAD)
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/download_linux_arm64 $(APP_DOWNLOAD)
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/download_windows_amd64.exe $(APP_DOWNLOAD)
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/download_mac_amd64 $(APP_DOWNLOAD)
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/download_mac_arm64 $(APP_DOWNLOAD)

.PHONY: static-query
static-query:
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/query_linux_amd64 $(APP_QUERY)
	GOOS=linux GOARCH=arm64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/query_linux_arm64 $(APP_QUERY)
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/query_windows_amd64.exe $(APP_QUERY)
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/query_mac_amd64 $(APP_QUERY)
	GOOS=darwin GOARCH=arm64 go build -ldflags "-X main.GitCommit=$(GIT_COMMIT)" -o bin/query_mac_arm64 $(APP_QUERY)

.PHONY: zip
zip: zip-linux zip-windows zip-mac

.PHONY: zip-linux
zip-linux:
	$(call generate_readme,Linux x86_64,linux_amd64)
	cd bin && zip commons-votes-linux-amd64.zip README.txt import_linux_amd64 download_linux_amd64 query_linux_amd64
	$(call generate_readme,Linux ARM64,linux_arm64)
	cd bin && zip commons-votes-linux-arm64.zip README.txt import_linux_arm64 download_linux_arm64 query_linux_arm64

.PHONY: zip-windows
zip-windows:
	$(call generate_readme,Windows x86_64,windows_amd64)
	cd bin && zip commons-votes-windows-amd64.zip README.txt import_windows_amd64.exe download_windows_amd64.exe query_windows_amd64.exe

.PHONY: zip-mac
zip-mac:
	$(call generate_readme,MacOS x86_64,mac_amd64)
	cd bin && zip commons-votes-mac-amd64.zip README.txt import_mac_amd64 download_mac_amd64 query_mac_amd64
	$(call generate_readme,MacOS ARM64,mac_arm64)
	cd bin && zip commons-votes-mac-arm64.zip README.txt import_mac_arm64 download_mac_arm64 query_mac_arm64

.PHONY: download
download:
	go run $(APP_DOWNLOAD) --start $(START_SESSION) --end $(END_SESSION)

.PHONY: import
import:
	go run $(APP_IMPORT)

.PHONY: clean
clean:
	rm -rf bin/
	rm -f $(DB_FILE)
	rm -f skipped_votes.log
	rm -f bin/*.zip

.PHONY: reset
reset:
	@time (make clean && make download && make import)

.PHONY: release
release: clean static-build zip
