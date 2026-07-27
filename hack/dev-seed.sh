#!/usr/bin/env bash
#
# Seeds the dev stack with demo workspaces, APIExports, and per-user RBAC so the
# marketplace's per-user filtering is visible in the SPA. Idempotent. Requires
# the stack to be up (hack/local-up.sh) and the admin kubeconfig.
#
# Demo users (all password "password", from config/dev/dex.yaml):
#   admin@example.com  access: demo, alpha   bind: widgets.example.io (+ public)
#   bob@example.com    access: alpha         bind: widgets.example.io (+ public)
#   alice@example.com  access: beta          bind: gadgets.example.io (+ public)
set -euo pipefail

cd "$(dirname "$0")/.."
KC="${KUBECONFIG_OUT:-$(pwd)/.kcp/admin.kubeconfig}"
FP="https://kcp.127.0.0.1.nip.io:8443"

root() { kubectl --kubeconfig "$KC" --context default "$@"; }             # /clusters/root
ws() { kubectl --kubeconfig "$KC" --context base --server="${FP}/clusters/root:$1" "${@:2}"; }

create_ws() {
	local name="$1"
	root apply -f - <<EOF >/dev/null
apiVersion: tenancy.kcp.io/v1alpha1
kind: Workspace
metadata:
  name: ${name}
EOF
	for _ in {1..40}; do
		[ "$(root get workspace "${name}" -o jsonpath='{.status.phase}' 2>/dev/null)" = Ready ] && return 0
		sleep 3
	done
	echo "workspace ${name} did not become Ready" >&2
	return 1
}

grant() { # workspace, users...
	local wsname="$1"
	shift
	local subs=""
	for u in "$@"; do
		subs+="  - kind: User
    name: ${u}
    apiGroup: rbac.authorization.k8s.io
"
	done
	ws "${wsname}" apply -f - <<EOF >/dev/null
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: marketplace-dev-access
subjects:
${subs}roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
EOF
}

export_api() { # workspace, group, plural, singular, Kind, exportName
	local wsname="$1" group="$2" plural="$3" singular="$4" kind="$5" exp="$6"
	# APIResourceSchemas are immutable, so create only if absent.
	if ! ws "${wsname}" get apiresourceschema "v1.${plural}.${group}" >/dev/null 2>&1; then
		ws "${wsname}" create -f - <<EOF >/dev/null
apiVersion: apis.kcp.io/v1alpha1
kind: APIResourceSchema
metadata:
  name: v1.${plural}.${group}
spec:
  group: ${group}
  names:
    kind: ${kind}
    listKind: ${kind}List
    plural: ${plural}
    singular: ${singular}
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              note:
                type: string
EOF
	fi
	ws "${wsname}" apply -f - <<EOF >/dev/null
apiVersion: apis.kcp.io/v1alpha2
kind: APIExport
metadata:
  name: ${exp}
spec:
  resources:
    - name: ${plural}
      group: ${group}
      schema: v1.${plural}.${group}
      storage:
        crd: {}
EOF
}

echo ">>> workspaces (root:demo, root:alpha, root:beta)"
create_ws demo
create_ws alpha
create_ws beta

echo ">>> APIExports (widgets in alpha, gadgets in beta)"
export_api alpha example.io widgets widget Widget widgets.example.io
export_api beta example.io gadgets gadget Gadget gadgets.example.io

echo ">>> per-user access/bind grants"
grant demo admin@example.com
grant alpha admin@example.com bob@example.com
grant beta alice@example.com

echo "seed done"
