.PHONY: build test lint clean generate protocol service frontend

build: protocol service frontend

generate: protocol

protocol:
	go install "github.com/bufbuild/buf/cmd/buf@latest"
	make -C protocol build

service: protocol
	make -C service build

frontend:
	make -wC frontend install build

test: protocol
	make -C service test

lint: protocol
	make -C service lint

clean:
	make -C protocol clean 2>/dev/null || true
	make -C service clean 2>/dev/null || true
	make -C frontend clean 2>/dev/null || true
