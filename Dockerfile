FROM registry.access.redhat.com/ubi8/go-toolset:1.16.12-4 AS builder

WORKDIR /github.com/redhat-appstudio/e2e-tests
USER root
COPY . .
RUN GOOS=linux make build

FROM scratch

WORKDIR /root/
COPY --from=builder /github.com/redhat-appstudio/e2e-tests/bin/e2e-appstudio ./
ENTRYPOINT ["/root/e2e-appstudio"]
