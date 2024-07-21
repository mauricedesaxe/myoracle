Version := $(shell git describe --tags --dirty)
GitCommit := $(shell git rev-parse HEAD)
LDFLAGS := "-s -w -X main.Version=$(Version) -X main.GitCommit=$(GitCommit)"

.PHONY: all
all: gofmt vendor dist

.PHONY: gofmt
gofmt:
	@test -z $(shell gofmt -l ./ | tee /dev/stderr) || (echo "[WARN] Fix formatting issues with 'make fmt'" && exit 1)

.PHONY: vendor
vendor:
	go mod vendor

.PHONY: dist
dist:
	mkdir -p bin/
	CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -ldflags $(LDFLAGS) -installsuffix cgo -o bin/myoracle-amd64
	CGO_ENABLED=0 GOOS=darwin go build -mod=vendor -a -ldflags $(LDFLAGS) -installsuffix cgo -o bin/myoracle-darwin
	GOARM=7 GOARCH=arm CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -ldflags $(LDFLAGS) -installsuffix cgo -o bin/myoracle-arm
	GOARCH=arm64 CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -ldflags $(LDFLAGS) -installsuffix cgo -o bin/myoracle-arm64
	GOOS=windows CGO_ENABLED=0 go build -mod=vendor -a -ldflags $(LDFLAGS) -installsuffix cgo -o bin/myoracle.exe

test:
	go test ./... -v -race 