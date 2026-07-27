.PHONY: run test build

run:
	go run ./src

test:
	go test -race ./...

build:
	mkdir -p bin
	go build -buildvcs=false -o bin/sshrpg ./src
