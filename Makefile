.PHONY: build build-mac build-mips test test-ble test-usb test-all test-e2e test-e2e-ui run clean deps lint lint-fix install-hooks

build:
	go build -o qlx ./cmd/qlx/

build-mac:
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
	  go build -tags ble,usb -o qlx-darwin ./cmd/qlx/

build-mips:
	CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat \
	  go build -tags minimal -trimpath -gcflags=all="-B" -ldflags="-s -w" \
	  -o qlx-mips ./cmd/qlx/

test:
	go test ./... -v

test-ble:
	go test -tags ble ./... -v

test-usb:
	go test -tags usb ./... -v

test-all:
	go test -tags ble,usb ./... -v

run:
	go run -tags ble ./cmd/qlx/ --port 18081 --data ./data

test-e2e:
	cd e2e && npx playwright test

test-e2e-ui:
	cd e2e && npx playwright test --ui

clean:
	rm -f qlx qlx-darwin qlx-mips qlx-e2e-test

deps:
	go mod download
	go mod tidy

lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

install-hooks:
	lefthook install
