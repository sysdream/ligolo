RELAY_SOURCE=./cmd/localrelay
LIGOLO_SOURCE=./cmd/ligolo
TLS_CERT ?= 'certs/cert.pem'
LDFLAGS="-s -w -X main.tlsFingerprint=$$(openssl x509 -fingerprint -sha256 -noout -in $(TLS_CERT) | cut -d '=' -f2)"
GCFLAGS="all=-trimpath=$GOPATH"

RELAY_BINARY=localrelay
LIGOLO_BINARY=ligolo
TAGS=release

OSARCH = "linux/amd64 linux/386 linux/arm windows/amd64 windows/386 darwin/amd64 darwin/386"

TLS_HOST ?= 'ligolo.lan'

.DEFAULT: help

help: ## Show Help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

dep: ## Install dependencies
	go get -d -v ./...
	go install -v github.com/mitchellh/gox@latest

certs: ## Build SSL certificates
	mkdir certs
	cd certs && go run `go env GOROOT`/src/crypto/tls/generate_cert.go -ecdsa-curve P256 -ed25519 -host $(TLS_HOST)

build: ## Build for the current architecture.
	go build -ldflags $(LDFLAGS) -gcflags $(GCFLAGS) -tags $(TAGS) -o bin/$(RELAY_BINARY) $(RELAY_SOURCE) && \
	go build -ldflags $(LDFLAGS) -gcflags $(GCFLAGS) -tags $(TAGS) -o bin/$(LIGOLO_BINARY) $(LIGOLO_SOURCE)

build-all: ## Build for every architectures.
	gox -osarch=$(OSARCH) -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -tags $(TAGS) -output "bin/$(LIGOLO_BINARY)_{{.OS}}_{{.Arch}}" $(LIGOLO_SOURCE)
	gox -osarch=$(OSARCH) -ldflags=$(LDFLAGS) -gcflags=$(GCFLAGS) -tags $(TAGS) -output "bin/$(RELAY_BINARY)_{{.OS}}_{{.Arch}}" $(RELAY_SOURCE)

clean:
	rm -rf certs
	rm bin/$(LIGOLO_BINARY)*
	rm bin/$(RELAY_BINARY)*
