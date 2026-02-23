.PHONY: build test lint vet clean

build:
	go build -o bin/jira-release-sync ./cmd/jira-release-sync

test:
	go test -race -cover ./...

vet:
	go vet ./...

lint: vet

clean:
	rm -rf bin/
