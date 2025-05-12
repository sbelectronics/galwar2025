all: build

.PHONY: build
build:
	go build -o build/_output/single=player ./cmd/single-player

.PHONY: go-format
go-format:
	go fmt $(shell sh -c "go list ./...")
