.PHONY: build test lint clean generate protocol service frontend go-tools

build: protocol service frontend

generate: protocol

go-tools:
	make -C service go-tools

protocol: go-tools
	make -C protocol build

service: protocol
	make -C service build

frontend:
	make -wC frontend build

test: protocol
	make -C service test

lint: protocol
	make -C service lint

clean:
	make -C protocol clean 2>/dev/null || true
	make -C service clean 2>/dev/null || true
	make -C frontend clean 2>/dev/null || true
