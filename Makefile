.PHONY: build build-mips test run clean deps

build:
	go build -o qlx ./cmd/qlx/

build-mips:
	CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat \
	  go build -trimpath -gcflags=all="-B" -ldflags="-s -w" \
	  -o qlx-mips ./cmd/qlx/

test:
	go test ./... -v

run:
	go run ./cmd/qlx/ --port 8080 --data ./data

clean:
	rm -f qlx qlx-mips

deps:
	go mod download
	go mod tidy
