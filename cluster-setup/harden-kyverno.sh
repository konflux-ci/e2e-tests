#!/bin/bash
# =============================================================================
# Harden Kyverno for burst PipelineRun workloads
#
# Problem: Konflux CI deploys Kyverno with absolute minimum resources:
#   - kyverno-admission-controller: cpu=1m, memory=1Mi (essentially zero)
#   - Single replica, no HA
#
# During a burst of 100+ pods (7+ concurrent PipelineRuns), the admission
# controller gets OOMKilled or starved. Since the webhook is fail-closed,
# ALL pod creation is blocked until Kyverno recovers — cascading PLR failures.
#
# This script should be run AFTER Konflux bootstrap completes and Kyverno
# is deployed. It:
#   1. Scales admission-controller to 3 replicas for HA
#   2. Increases resources so it can handle burst webhook traffic
#   3. Scales background-controller to 2 replicas
#   4. Disables reports-controller (ephemeral reports add massive etcd churn)
#
# Usage:
#   ./harden-kyverno.sh [KUBECONFIG]
#
# Reference:
#   https://github.com/konflux-ci/konflux-ci/blob/main/dependencies/kyverno/kustomization.yaml
# =============================================================================

set -euo pipefail

KUBECONFIG="${1:-./kubeconfig}"
export KUBECONFIG

echo "==> Hardening Kyverno for burst workloads..."

# Wait for Kyverno to exist
if ! kubectl get namespace kyverno &>/dev/null; then
  echo "    Kyverno namespace not found. Run this after Konflux bootstrap."
  exit 1
fi

if ! kubectl get deployment kyverno-admission-controller -n kyverno &>/dev/null; then
  echo "    kyverno-admission-controller not found. Skipping."
  exit 1
fi

# --- Scale admission controller to 3 replicas ---
echo "    Scaling admission-controller to 3 replicas..."
kubectl scale deployment kyverno-admission-controller -n kyverno --replicas=3

# --- Increase admission controller resources ---
# From: cpu=1m, memory=1Mi (Konflux default — way too low for burst)
# To:   requests cpu=100m/memory=256Mi, limits memory=1Gi
echo "    Increasing admission-controller resources..."
kubectl patch deployment kyverno-admission-controller -n kyverno --type='json' -p='[
  {"op":"replace","path":"/spec/template/spec/containers/0/resources/requests/cpu","value":"100m"},
  {"op":"replace","path":"/spec/template/spec/containers/0/resources/requests/memory","value":"256Mi"},
  {"op":"replace","path":"/spec/template/spec/containers/0/resources/limits/memory","value":"1Gi"}
]'

# --- Scale background controller to 2 replicas ---
echo "    Scaling background-controller to 2 replicas..."
kubectl scale deployment kyverno-background-controller -n kyverno --replicas=2 2>/dev/null || true

# --- Increase background controller resources ---
kubectl patch deployment kyverno-background-controller -n kyverno --type='json' -p='[
  {"op":"replace","path":"/spec/template/spec/containers/0/resources/requests/cpu","value":"50m"},
  {"op":"replace","path":"/spec/template/spec/containers/0/resources/requests/memory","value":"128Mi"},
  {"op":"replace","path":"/spec/template/spec/containers/0/resources/limits/memory","value":"512Mi"}
]' 2>/dev/null || true

# --- Disable ephemeral reports (massive etcd write churn, useless for E2E) ---
# Kyverno generates ephemeralreports + clusterephemeralreports for every
# resource it evaluates. During burst, this adds 100-200+ etcd objects with
# constant create/update churn. Zero value for E2E tests.
# Fix: scale reports-controller to 0 and delete existing reports.
echo "    Disabling ephemeral reports (etcd write churn reduction)..."
kubectl scale deployment kyverno-reports-controller -n kyverno --replicas=0 2>/dev/null || true
kubectl delete ephemeralreports.reports.kyverno.io --all -A --timeout=30s 2>/dev/null || true
kubectl delete clusterephemeralreports.reports.kyverno.io --all --timeout=30s 2>/dev/null || true
echo "    Reports controller scaled to 0, existing reports purged."

# --- Wait for rollout ---
echo "    Waiting for rollout..."
kubectl rollout status deployment/kyverno-admission-controller -n kyverno --timeout=120s

echo ""
echo "==> Kyverno hardened:"
echo "    admission-controller: 3 replicas, 256Mi-1Gi memory, 100m CPU"
echo "    background-controller: 2 replicas, 128Mi-512Mi memory, 50m CPU"
echo "    reports-controller: DISABLED (0 replicas — no etcd churn)"
echo ""
echo "    Status:"
kubectl get pods -n kyverno -o wide
echo ""
