VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: all web server test run dev clean

all: web server

web:
	cd web && npm ci && npx ng build

server:
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/guntacontrol ./cmd/guntacontrol

test:
	go vet ./...
	go test ./...

# server with embedded UI on http://localhost:8080 (needs `make web` first)
run: server
	bin/guntacontrol -allow-write

# Angular dev server with live reload (proxies /api to :8080)
dev:
	cd web && npx ng serve

clean:
	rm -rf bin web/dist
