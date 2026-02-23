#!/bin/bash

set -o nounset
set -o errexit
set -o pipefail

if [[ -z "${GITHUB_ORG:-}" ]]; then
    echo "ERROR: No GITHUB_ORG variable provided" >&2
    exit 1
fi

if [[ -z "${SMEE_CHANNEL:-}" ]]; then
    echo "ERROR: No SMEE_CHANNEL variable provided" >&2
    exit 1
fi

if ! oc -n openshift-pipelines get service/pipelines-as-code-controller -o name >/dev/null; then
    echo "ERROR: You are either not logged in to OpenShift cluster of service/pipelines-as-code-controller does not exist that might indicate Konflux was not installed yet" >&2
    exit 1
fi

cat << EOF | oc apply -f -
---
apiVersion: v1
kind: Namespace
metadata:
  name: smee-client
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gosmee-client
  namespace: smee-client
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gosmee-client
  template:
    metadata:
      labels:
        app: gosmee-client
    spec:
      containers:
        - image: "ghcr.io/chmouel/gosmee:v0.21.0"
          imagePullPolicy: Always
          name: gosmee
          args:
            - "client"
            - https://smee.io/${SMEE_CHANNEL}
            - "http://pipelines-as-code-controller.openshift-pipelines:8080"
          securityContext:
            readOnlyRootFilesystem: true
            runAsNonRoot: true
          resources:
            limits:
              cpu: 100m
              memory: 32Mi
            requests:
              cpu: 10m
              memory: 32Mi
EOF

oc -n smee-client rollout status deployment/gosmee-client

echo "Now go to https://github.com/organizations/${GITHUB_ORG}/settings/apps and setup a webhook URL to https://smee.io/${SMEE_CHANNEL}"
