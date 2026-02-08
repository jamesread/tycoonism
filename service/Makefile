.PHONY: build test lint run clean protocol

protocol:
	$(MAKE) -C ../protocol build

build: protocol
	go build -o bin/server .

test:
	go test ./...

lint:
	go vet ./...
	golangci-lint run ./... 2>/dev/null || true

run: build
	./bin/server

clean:
	rm -rf bin
