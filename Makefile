all: build

.PHONY: build
build:
	go build -o build/_output/single-player ./cmd/single-player
	go build -o build/_output/galwar-server ./cmd/galwar-server

.PHONY: go-format
go-format:
	go fmt $(shell sh -c "go list ./...")

# Cross-compiles the server for a Linux/amd64 host.
.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o build/_output/galwar-server-linux ./cmd/galwar-server
