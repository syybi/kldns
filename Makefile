APP_NAME := kldns
SERVER_PACKAGE := ./cmd/server

.PHONY: run test build web-dev web-check web-build clean

run:
	go run $(SERVER_PACKAGE)

test:
	go test ./...

web-dev:
	npm --prefix web run dev

web-check:
	npm --prefix web run typecheck
	npm --prefix web run lint

web-build:
	npm --prefix web run build

build:
	pwsh -File ./scripts/build.ps1

clean:
	go clean
