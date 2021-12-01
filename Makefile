BINARY_NAME := ./bin/e2e-appstudio
CONTAINER_TAG ?= next
CONTAINER_IMAGE_NAME := quay.io/redhat-appstudio/e2e:$(CONTAINER_TAG)

build:
	go mod vendor && CGO_ENABLED=0 go test -v -c -o $(BINARY_NAME) ./cmd/e2e_test.go

build-container:
	podman build -t $(CONTAINER_IMAGE_NAME) --no-cache .

push-container:
	podman push $(CONTAINER_IMAGE_NAME)
