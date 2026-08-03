.PHONY: all build test clean

BINARY_NAME=qmd
BUILD_TAGS=sqlite_fts5
GO_FLAGS=-tags "$(BUILD_TAGS)" -mod=vendor

all: build

build:
	go build $(GO_FLAGS) -o $(BINARY_NAME) ./cmd/qmd

test:
	go test $(GO_FLAGS) -v ./...

clean:
	rm -f $(BINARY_NAME)
