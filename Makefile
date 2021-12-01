OUT_FILE := ./bin/e2e_appstudio
DOCKER_IMAGE_NAME :=quay.io/flacatus/e2e:next

build:
	go mod vendor && CGO_ENABLED=0 go test -v -c -o ${OUT_FILE} ./cmd/e2e_test.go

build-container:
	podman build -t $(DOCKER_IMAGE_NAME) --no-cache .

push-container:
	podman push $(DOCKER_IMAGE_NAME)
