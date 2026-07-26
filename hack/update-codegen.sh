#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")/.."

CONTROLLER_GEN="${CONTROLLER_GEN:-controller-gen}"

"$CONTROLLER_GEN" \
	object:headerFile=hack/boilerplate.go.txt \
	paths=./apis/marketplace/v1alpha1/...

mkdir -p pkg/generated/openapi
go tool k8s.io/kube-openapi/cmd/openapi-gen \
	--output-dir ./pkg/generated/openapi \
	--output-pkg github.com/ntnn/kcp-marketplace/pkg/generated/openapi \
	--output-file zz_generated.openapi.go \
	--go-header-file hack/boilerplate.go.txt \
	github.com/ntnn/kcp-marketplace/apis/marketplace/v1alpha1 \
	k8s.io/apimachinery/pkg/apis/meta/v1 \
	k8s.io/apimachinery/pkg/runtime \
	k8s.io/apimachinery/pkg/version
