E2E_BIN := ./bin/e2e-appstudio
E2E_ARGS_EXEC ?= ""
CONTAINER_TAG ?= next
CONTAINER_IMAGE_NAME ?= quay.io/redhat-appstudio/e2e:$(CONTAINER_TAG)

.PHONY: vendor/check
vendor/check: vendor/fix
	git diff --exit-code vendor/ go.*

.PHONY: vendor/fix
vendor/fix:
	go mod tidy -compat=1.19
	go mod vendor

build:
	CGO_ENABLED=0 GOFLAGS=-mod=vendor go test -v -c -o $(E2E_BIN) ./cmd/e2e_test.go

build-container:
	podman build -t $(CONTAINER_IMAGE_NAME) .

push-container:
	podman push $(CONTAINER_IMAGE_NAME)

run:
	$(E2E_BIN) $(E2E_ARGS_EXEC)

test/unit:
	go test -v ./pkg/... ./magefiles/...

ci/test/e2e:
	./mage -v ci:teste2e

ci/test/upgrade:
	./mage -v ci:testUpgrade 

ci/prepare/e2e-branch:
	./mage -v ci:prepareE2Ebranch

local/cluster/prepare:
	./mage -v local:prepareCluster

local/cluster/upgrade:
	./mage -v local:testUpgrade

local/test/e2e:
	./mage -v local:teste2e

local/template/generate-test-suite:
	./mage -v local:generateTestSuiteFile

local/template/generate-test-spec:
	./mage -v local:generateTestSpecFile

clean-gitops-repositories:
	DRY_RUN=false ./mage -v local:cleanupGithubOrg

clean-github-webhooks:
	./mage -v cleanWebHooks

clean-quay-repos-and-robots:
	./mage -v local:cleanupQuayReposAndRobots

clean-quay-tags:
	./mage -v local:cleanupQuayTags