BINARY := chatserver
PKGS   := ./...

.PHONY: all build run test race bench vet fmt lint cert docker clean

all: fmt vet test

build:
	go build -trimpath -ldflags="-s -w" -o bin/$(BINARY) ./cmd/chatserver

run:
	go run ./cmd/chatserver

test:
	go test $(PKGS)

race:
	go test -race $(PKGS)

bench:
	go test -run '^$$' -bench . -benchmem $(PKGS)

vet:
	go vet $(PKGS)

fmt:
	gofmt -l -w .

lint: fmt vet

cert:
	go run ./cmd/gencert -cert cert.pem -key key.pem

docker:
	docker build -t chatserver:latest .

clean:
	rm -rf bin cert.pem key.pem
