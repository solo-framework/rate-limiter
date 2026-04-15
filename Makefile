.DEFAULT_GOAL := help
.PHONY: *

VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-list --max-count=1 --abbrev-commit $(VERSION) 2>/dev/null || echo "none")
COMMIT_DATE := $(shell git show -s --format=%cI $(COMMIT) 2>/dev/null || echo "none")
DOCKER_BUILD_DIR := ./build/docker
LOCAL_BUILD_DIR := ./build/local

help:
	@echo "Available commands:"
	@echo "  help               - Show this help message"
	@echo "  test_all           - Run all tests with race detector and coverage"
	@echo "  test_all_cover     - Run all tests and open HTML coverage report"
	@echo "  test_service       - Run service package tests with race detector and coverage"
	@echo "  test_service_cover - Run service package tests and open HTML coverage report"
	@echo "  test_control       - Run control package tests with race detector and coverage"
	@echo "  test_control_cover - Run control package tests and open HTML coverage report"
# 	@echo "  run                - Run the application with config from config.toml and ENV variables (see the run for details)"
	@echo "  run-default        - Run the application with default settings"
	@echo "  ea                 - Run escape analysis and write output to ea.txt"
	@echo "  bench              - Run Benchmark_GetLimiter with CPU/Memory profiles"
	@echo "  ab                 - Run load test utility from ./cmd/ab"
	@echo "  build-in-docker 	- Build Linux binary in docker"
	@echo "  run-in-docker 	    - Run service in docker"


# go list ./... | grep -vE '(^|/)cmd($|/)'
test_all:
	@go test -cover -count=1 -race  ./...

test_all_cover:
	@go test -cover -count=1 -v \
		-race \
		-coverprofile=test_all_cover.out \
		-covermode=atomic \
		-coverpkg=./... \
		./control/...  ./service/... ./internal/... \
	&& go tool cover -html=test_all_cover.out

test_service:
	go test -cover -count=1 -v -race ./service/...

test_service_cover:
	@go test -v -cover -count=1 -race \
		-coverprofile=test_service_cover.out \
		-covermode=atomic \
		-coverpkg=ratelimiter/service,ratelimiter/service/tests \
		./service/... &&\
	go tool cover -html=test_service_cover.out

test_control:
	@go test -cover -count=1 -v -race ./control/...

test_control_cover:
	@go test -v -cover -count=1 -race \
		-coverprofile=test_control_cover.out \
		-covermode=atomic \
		-coverpkg=ratelimiter/control,ratelimiter/control/tests \
		./control/... &&\
	go tool cover -html=test_control_cover.out

# run:
# 	@[ ! -e "./config.toml" ] &&  echo "copy dist.config.toml to config.toml and try again" && exit 1 ||  \
# 	 	RATE_LIMITER_ENV=dev RATE_LIMITER_HTTP_PORT=8090 RATE_LIMITER_HTTP_SECRET=secret go run -race ./cmd/main.go --config ./config.toml

run-default:
	@[ ! -e "./config.toml" ] &&  echo "copy dist.config.toml to config.toml and try again" && exit 1 || \
	RATE_LIMITER_ENV=dev go run -race ./cmd/main.go --config ./config.toml


run-in-docker:
	@[ ! -e ".env" ] && echo "rename dist.env to .env" && cp ./dist.env .env; \
	[ ! -e "./config.toml" ] && echo "rename dist.config.toml" && cp ./dist.config.toml ./config.toml; \
		docker compose -f ./docker-compose.yaml build \
			--build-arg VERSION="$(VERSION)" \
			--build-arg COMMIT="$(COMMIT)" \
			--build-arg DATE="$(COMMIT_DATE)" \
		&& docker compose -f ./docker-compose.yaml up -d

build:
	@go install github.com/golang/mock/mockgen@v1.6.0 \
		&& go install go.uber.org/mock/mockgen@latest \
		&& go install golang.org/x/vuln/cmd/govulncheck@latest \
		&& go mod download \
		&& go mod tidy \
		&& govulncheck ./... \
		&& rm -rf $(LOCAL_BUILD_DIR) && mkdir -p $(LOCAL_BUILD_DIR) \
		&& CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
			go build \
				-trimpath \
				-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" \
				-o $(LOCAL_BUILD_DIR)/ratelimiter ./cmd \
		&& cp ./dist.config.toml $(LOCAL_BUILD_DIR)/config.toml

build-in-docker:
	@docker buildx build \
	 	--platform linux/amd64 \
		--file ./docker/build.Dockerfile \
		--build-arg VERSION="$(VERSION)" \
		--build-arg COMMIT="$(COMMIT)" \
		--build-arg DATE="$(COMMIT_DATE)" \
		--output type=local,dest=$(DOCKER_BUILD_DIR) \
		--tag rate-limiter:"$(VERSION)" . \
	&& echo "built into dir $(DOCKER_BUILD_DIR)"

#  (Escape Analysis)
ea:
	@go build -gcflags "-m" ./... 2> ea.txt

bench:
	@go test ./service/tests/manager_benchmark_test.go ./service/tests/config.go ./service/tests/helpers.go -bench=Benchmark_GetLimiter -benchmem -cpuprofile=./pprof/cpu.out -memprofile=./pprof/mem.out -count=4 -benchtime=100000x

ab:
	@go run ./cmd/ab/ab.go

