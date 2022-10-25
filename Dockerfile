FROM registry.ci.openshift.org/openshift/release:golang-1.18 AS builder

WORKDIR /github.com/redhat-appstudio/e2e-tests
USER root
COPY . .
RUN GOOS=linux make build

FROM registry.access.redhat.com/ubi8/ubi-minimal:latest

WORKDIR /root/
COPY --from=builder /github.com/redhat-appstudio/e2e-tests/bin/e2e-appstudio ./
COPY --from=builder /github.com/redhat-appstudio/e2e-tests/tests ./tests
ENTRYPOINT ["/root/e2e-appstudio"]
