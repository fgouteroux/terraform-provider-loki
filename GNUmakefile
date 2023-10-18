LOKI_VERSION ?= 2.8.3

default: build

build:
	go build -v ./...

install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}

# See https://golangci-lint.run/
lint:
	golangci-lint run -c .golangci.toml ./...

generate:
	go generate ./...

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout 30s -coverprofile=cover.out -parallel=4 ./...
	go tool cover -func=cover.out

testacc: compose-up
	curl -s --retry 12 -f --retry-all-errors --retry-delay 10 http://localhost:3100/ready
	TF_ACC=1 LOKI_URI=http://localhost:3100 go test -cover ./... -v $(TESTARGS) -timeout 120m -coverprofile=cover.out
	go tool cover -func=cover.out

compose-up: compose-down
	LOKI_VERSION=$(LOKI_VERSION) docker-compose -f ./docker-compose.yml up -d

compose-down:
	docker-compose -f ./docker-compose.yml stop

.PHONY: build install lint generate fmt test testacc