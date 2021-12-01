FROM golang:1.16 AS builder

WORKDIR /github.com/redhat-appstudio/e2e-tests
COPY . .
RUN GOOS=linux make build

FROM scratch

WORKDIR /root/
COPY --from=builder /github.com/redhat-appstudio/e2e-tests/bin/e2e_appstudio ./
ENTRYPOINT ["/root/e2e_appstudio"]
