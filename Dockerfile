FROM registry.ci.openshift.org/openshift/release:golang-1.20 AS builder

WORKDIR /github.com/redhat-appstudio/e2e-tests
USER root

COPY go.mod .
COPY go.sum .
RUN go mod download -x
COPY cmd/ cmd/
COPY magefiles/ magefiles/
COPY pkg/ pkg/
COPY tests/ tests/
COPY Makefile .

RUN make build

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

WORKDIR /root/
COPY --from=builder /github.com/redhat-appstudio/e2e-tests/bin/e2e-appstudio ./
COPY --from=builder /github.com/redhat-appstudio/e2e-tests/tests ./tests
ENTRYPOINT ["/root/e2e-appstudio"]
