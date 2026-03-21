.PHONY: build build-mac build-mips test test-ble run clean deps

build:
	go build -o qlx ./cmd/qlx/

build-mac:
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
	  go build -tags ble -o qlx-darwin ./cmd/qlx/

build-mips:
	CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat \
	  go build -tags minimal -trimpath -gcflags=all="-B" -ldflags="-s -w" \
	  -o qlx-mips ./cmd/qlx/

test:
	go test ./... -v

test-ble:
	go test -tags ble ./... -v

run:
	go run ./cmd/qlx/ --port 8080 --data ./data

clean:
	rm -f qlx qlx-darwin qlx-mips

deps:
	go mod download
	go mod tidy
