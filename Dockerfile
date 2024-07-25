FROM registry.ci.openshift.org/openshift/release:golang-1.21 AS builder

WORKDIR /github.com/konflux-ci/e2e-tests
USER root

ARG ORAS_VERSION=1.2.0

RUN curl -LO "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_amd64.tar.gz" && \
    mkdir -p oras-install/ && \
    tar -zxf oras_${ORAS_VERSION}_*.tar.gz -C oras-install/ && \
    mv oras-install/oras /usr/local/bin/ && \
    rm -rf oras_${ORAS_VERSION}_*.tar.gz oras-install/ && \
    oras version

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
COPY --from=builder /github.com/konflux-ci/e2e-tests/bin/e2e-appstudio ./
COPY --from=builder /github.com/konflux-ci/e2e-tests/tests ./tests
COPY --from=builder /usr/local/bin/oras /usr/local/bin/oras
ENTRYPOINT ["/root/e2e-appstudio"]
