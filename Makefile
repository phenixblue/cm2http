PROJECT?=github.com/phenixblue/cm2http
APP?=cm2http
PORT?=5555

RELEASE?=v0.0.1
COMMIT?=$(shell git rev-parse --short HEAD)
BUILD_TIME?=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
CONTAINER_IMAGE?=docker.io/thewebroot/${APP}

GOOS?=linux
GOARCH?=amd64

clean:
	rm -f ${APP}

build: clean
	CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build \
		-ldflags "-s -w -X ${PROJECT}/version.Release=${RELEASE} \
		-X ${PROJECT}/version.Commit=${COMMIT} -X ${PROJECT}/version.BuildTime=${BUILD_TIME}" \
		-o bin/${APP}

container: build
	docker build -t $(CONTAINER_IMAGE):$(RELEASE) .

run: container
	docker stop $(APP):$(RELEASE) || true && docker rm $(APP):$(RELEASE) || true
	docker run --name ${APP} -p ${PORT}:${PORT} --rm \
		-e "PORT=${PORT}" \
		$(APP):$(RELEASE)

test:
	go test -v -race ./...

push: container
	docker push $(CONTAINER_IMAGE):$(RELEASE)

install:
	kustomize build deploy/overlays/standard | kubectl apply -f -

install-standalone:
	namespace="$(namespace)"
	kustomize build deploy/overlays/standalone | kubectl apply -n "$(namespace)" -f -
