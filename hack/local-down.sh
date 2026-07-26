#!/usr/bin/env bash
# Tear down the kind stack created by local-up.sh.
set -euo pipefail

cd "$(dirname "$0")/.."

CLUSTER=kcp-marketplace

kind delete cluster --name "${CLUSTER}"
rm -rf .kcp
