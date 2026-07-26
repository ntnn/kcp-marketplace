#!/usr/bin/env bash
#
# https://nip.io/ / https://sslip.io/ provide free hostname/ip mappings.
# Safer than x.localhost.
#
# Endpoints (all resolve to 127.0.0.1 via nip.io, reachable at :8443):
#   front-proxy   https://kcp.127.0.0.1.nip.io:8443
#   root shard    https://root.kcp.127.0.0.1.nip.io:8443
#   theseus shard https://theseus.kcp.127.0.0.1.nip.io:8443
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

CLUSTER=kcp-marketplace
NS=kcp
IMG=kcp-marketplace-vws:dev
OP=config/dev/operator
KUBECONFIG_OUT="${ROOT}/.kcp/admin.kubeconfig"

wait_deploy() { # namespace, name
	local ns="$1" name="$2"
	for i in {1..80}; do
		kubectl -n "${ns}" get deploy "${name}" >/dev/null 2>&1 && break
		sleep 3
	done
	kubectl -n "${ns}" rollout status "deploy/${name}" --timeout=240s
}

echo ">>> [1/8] kind cluster"
if ! kind get clusters 2>/dev/null | grep -qx "${CLUSTER}"; then
	kind create cluster --config config/dev/kind.yaml
fi
kubectl config use-context "kind-${CLUSTER}" >/dev/null

echo ">>> [2/8] cert-manager"
helm repo add jetstack https://charts.jetstack.io >/dev/null 2>&1 || true
helm repo update jetstack >/dev/null
helm upgrade --install cert-manager jetstack/cert-manager \
	--namespace cert-manager --create-namespace \
	--set crds.enabled=true --wait --timeout 5m

echo ">>> [3/8] envoy gateway + Gateway (TLS passthrough, NodePort 31443)"
helm upgrade --install envoy oci://registry-1.docker.io/envoyproxy/gateway-helm \
	--version v1.7.0 --namespace envoy-gateway-system --create-namespace --wait --timeout 5m
kubectl apply -f "${OP}/gateway.yaml"

echo ">>> [4/8] kcp-operator + issuer + etcd"
kubectl apply -k "${OP}/kcp-operator"
kubectl -n kcp-operator-system rollout status deploy/kcp-operator-controller-manager --timeout=180s
kubectl apply -f "${OP}/issuer.yaml" -f "${OP}/etcd.yaml"

echo ">>> [5/8] RootShard + theseus Shard + FrontProxy + routes + Kubeconfig"
kubectl apply -f "${OP}/rootshard.yaml"
for i in {1..80}; do
	[ "$(kubectl -n "${NS}" get rootshard root -o jsonpath='{.status.phase}' 2>/dev/null)" = "Running" ] && break
	sleep 5
done
kubectl apply -f "${OP}/shard-theseus.yaml" -f "${OP}/frontproxy.yaml"
wait_deploy "${NS}" root-kcp
wait_deploy "${NS}" theseus-shard-kcp
wait_deploy "${NS}" frontproxy-front-proxy

echo ">>> [6/8] build vws image + load into kind"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/access-vws ./cmd/access-vws
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/apiexport-vws ./cmd/apiexport-vws
docker build -f config/dev/Dockerfile -t "${IMG}" .
kind load docker-image "${IMG}" --name "${CLUSTER}"

echo ">>> [7/8] marketplace virtual workspaces"
kubectl apply -f config/dev/marketplace.yaml
wait_deploy "${NS}" marketplace-access-vws
wait_deploy "${NS}" marketplace-apiexport-vws
# The front-proxy loads the mapping file at startup; ensure the current mappings
# (and VW backends) are live.
kubectl -n "${NS}" rollout restart deploy/frontproxy-front-proxy
kubectl -n "${NS}" rollout status deploy/frontproxy-front-proxy --timeout=120s

echo ">>> [8/8] host admin kubeconfig -> ${KUBECONFIG_OUT}"
mkdir -p "$(dirname "${KUBECONFIG_OUT}")"
kubectl -n "${NS}" wait kubeconfig/frontproxy --for=condition=Available --timeout=120s >/dev/null
# The operator kubeconfig ships two contexts: "default" targets .../clusters/root,
# "base" targets the bare front-proxy origin. The marketplace path-mappings live at
# the bare origin, so select "base".
kubectl -n "${NS}" get secret kcp-frontproxy-kubeconfig -o jsonpath='{.data.kubeconfig}' | base64 -d > "${KUBECONFIG_OUT}"
kubectl --kubeconfig "${KUBECONFIG_OUT}" config use-context base >/dev/null

cat <<EOF

Stack is up (kcp-operator, 2 shards + front-proxy via envoy gateway).

  front-proxy:   https://kcp.127.0.0.1.nip.io:8443
  root shard:    https://root.kcp.127.0.0.1.nip.io:8443
  theseus shard: https://theseus.kcp.127.0.0.1.nip.io:8443

  export KUBECONFIG=${KUBECONFIG_OUT}
  kubectl get --raw '/services/marketplace-access/apis/marketplace.kcp.io/v1alpha1/accessibleworkspaces' | jq
  kubectl get --raw '/services/marketplace-apiexports/apis/marketplace.kcp.io/v1alpha1/bindableapiexports' | jq

Tear down with: make down
EOF
