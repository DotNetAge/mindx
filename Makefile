.PHONY: build test lint clean run tidy

BINARY_NAME=mindx
GO=go
GOFLAGS=-trimpath
LDFLAGS=-s -w

build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) .

test:
	$(GO) test -race -coverprofile=coverage.out ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out

run: build
	./$(BINARY_NAME) start

tidy:
	$(GO) mod tidy
