.PHONY: run test vet vulncheck test-postgres check build

run:
	go run ./src

test:
	go test -race ./...

vet:
	go vet ./...

vulncheck:
	govulncheck ./...

test-postgres:
	test -n "$$POSTGRES_TEST_DATABASE_URL"
	go test -race -count=1 ./src/repository -run '^TestPostgreSQL'

check: test vet vulncheck

build:
	mkdir -p bin
	go build -buildvcs=false -o bin/sshrpg ./src
