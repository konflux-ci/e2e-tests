FROM registry.access.redhat.com/ubi9/go-toolset:1.25 AS builder
ENV GOBIN=$HOME/bin

USER root

# renovate: datasource=repology depName=homebrew/openshift-cli
ARG OC_VERSION=4.14.8

# renovate: datasource=github-releases depName=jqlang/jq
ARG JQ_VERSION=1.7.1

# renovate: datasource=github-releases depName=mikefarah/yq
ARG YQ_VERSION=4.43.1

# renovate: datasource=github-releases depName=oras-project/oras
ARG ORAS_VERSION=1.2.0

ARG TARGETARCH

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

RUN ARCH=${TARGETARCH} && \
    if [ "$ARCH" = "amd64" ]; then \
        OC_SUFFIX=""; \
    elif [ "$ARCH" = "arm64" ]; then \
        OC_SUFFIX="-arm64"; \
    else \
        echo "Unsupported architecture: $ARCH" && exit 1; \
    fi && \
    curl -L "https://mirror.openshift.com/pub/openshift-v4/clients/ocp/${OC_VERSION}/openshift-client-linux${OC_SUFFIX}.tar.gz" -o /tmp/openshift-client-linux.tar.gz && \
    tar --no-same-owner -xzf /tmp/openshift-client-linux.tar.gz && \
    mv oc kubectl /usr/local/bin && \
    oc version --client && \
    kubectl version --client

RUN ARCH=${TARGETARCH} && \
    if [ "$ARCH" = "amd64" ]; then \
        JQ_ARCH="amd64"; \
    elif [ "$ARCH" = "arm64" ]; then \
        JQ_ARCH="arm64"; \
    else \
        echo "Unsupported architecture: $ARCH" && exit 1; \
    fi && \
    curl -L "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/jq-linux-${JQ_ARCH}" -o /usr/local/bin/jq && \
    chmod +x /usr/local/bin/jq && \
    jq --version

RUN ARCH=${TARGETARCH} && \
    if [ "$ARCH" = "amd64" ]; then \
        YQ_ARCH="amd64"; \
    elif [ "$ARCH" = "arm64" ]; then \
        YQ_ARCH="arm64"; \
    else \
        echo "Unsupported architecture: $ARCH" && exit 1; \
    fi && \
    curl -L "https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_${YQ_ARCH}" -o /usr/local/bin/yq && \
    chmod +x /usr/local/bin/yq && \
    yq --version

RUN ARCH=${TARGETARCH} && \
    if [ "$ARCH" = "amd64" ]; then \
        ORAS_ARCH="amd64"; \
    elif [ "$ARCH" = "arm64" ]; then \
        ORAS_ARCH="arm64"; \
    else \
        echo "Unsupported architecture: $ARCH" && exit 1; \
    fi && \
    curl -LO "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/oras_${ORAS_VERSION}_linux_${ORAS_ARCH}.tar.gz" && \
    mkdir -p oras-install/ && \
    tar -zxf oras_${ORAS_VERSION}_*.tar.gz -C oras-install/ && \
    mv oras-install/oras /usr/local/bin/ && \
    rm -rf oras_${ORAS_VERSION}_*.tar.gz oras-install/ && \
    oras version

FROM registry.access.redhat.com/ubi9/go-toolset:1.25
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
