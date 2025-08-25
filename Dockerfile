FROM quay.io/konflux-ci/tekton-integration-catalog/sealights-go:latest as sealights-agents
FROM registry.access.redhat.com/ubi9/go-toolset:1.24 AS builder
ENV GOBIN=$HOME/bin

USER root

# renovate: datasource=repology depName=homebrew/openshift-cli
ARG OC_VERSION=4.14.8

# renovate: datasource=github-releases depName=stedolan/jq
ARG JQ_VERSION=1.6

# renovate: datasource=github-releases depName=mikefarah/yq
ARG YQ_VERSION=4.43.1

# renovate: datasource=github-releases depName=oras-project/oras
ARG ORAS_VERSION=1.2.0

WORKDIR /konflux-e2e

COPY go.mod .
COPY go.sum .
RUN go mod download -x
COPY cmd/ cmd/
COPY magefiles/ magefiles/
COPY pkg/ pkg/
COPY tests/ tests/

RUN go install -mod=mod github.com/onsi/ginkgo/v2/ginkgo
RUN ginkgo build ./cmd

RUN curl -L "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${OC_VERSION}/openshift-client-linux.tar.gz" -o /tmp/openshift-client-linux.tar.gz && \
    tar --no-same-owner -xzf /tmp/openshift-client-linux.tar.gz && \
    mv oc kubectl /usr/local/bin && \
    oc version --client && \
    kubectl version --client

RUN curl -L "https://github.com/stedolan/jq/releases/download/jq-${JQ_VERSION}/jq-linux64" -o /usr/local/bin/jq  && \
    chmod +x /usr/local/bin/jq && \
    jq --version

RUN curl -L "https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_amd64" -o /usr/local/bin/yq && \
    chmod +x /usr/local/bin/yq && \
    yq --version

RUN curl -LO "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_amd64.tar.gz" && \
    mkdir -p oras-install/ && \
    tar -zxf oras_${ORAS_VERSION}_*.tar.gz -C oras-install/ && \
    mv oras-install/oras /usr/local/bin/ && \
    rm -rf oras_${ORAS_VERSION}_*.tar.gz oras-install/ && \
    oras version

FROM registry.access.redhat.com/ubi9/go-toolset:1.24
USER root

WORKDIR /konflux-e2e
RUN chmod -R 775 /konflux-e2e

ENV GOBIN=$HOME/bin
ENV E2E_BIN_PATH=/konflux-e2e/konflux-e2e.test

RUN chmod -R 775 $HOME

RUN dnf -y install skopeo

COPY --from=builder /usr/local/bin/jq /usr/local/bin/jq
COPY --from=builder /usr/local/bin/yq /usr/local/bin/yq
COPY --from=builder /usr/local/bin/oc /usr/local/bin/oc
COPY --from=builder /usr/local/bin/kubectl /usr/local/bin/kubectl
COPY --from=builder /usr/local/bin/oras /usr/local/bin/oras
COPY --from=builder $GOBIN/ginkgo /usr/local/bin
COPY --from=builder /konflux-e2e/cmd/cmd.test konflux-e2e.test
COPY --from=sealights-agents /usr/local/bin/slgoagent /usr/local/bin/slgoagent
COPY --from=sealights-agents /usr/local/bin/slcli /usr/local/bin/slcli