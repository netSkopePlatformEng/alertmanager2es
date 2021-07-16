
#!/bin/bash
set -o errexit
set -o pipefail
set -o nounset

: ${GKE_KUBECONFIG:?"Need to set GKE_KUBECONFIG"}

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
kubectl create ns logging-system > /dev/null 2>&1 || true
kubectl --kubeconfig ${GKE_KUBECONFIG} get secret elasticsearch-es-elastic-user --namespace=elastic  -o json | jq 'del(.metadata.creationTimestamp)' | jq 'del(.metadata.ownerReferences)' | jq 'del(.metadata.namespace)' | jq 'del(.metadata.selfLink)' | jq 'del(.metadata.uid)' | jq 'del(.metadata.resourceVersion)' | kubectl apply --namespace=cattle-monitoring-system -f -
kubectl --kubeconfig ${GKE_KUBECONFIG} get secret elasticsearch-es-http-certs-public --namespace=elastic  -o json | jq 'del(.metadata.creationTimestamp)' | jq 'del(.metadata.ownerReferences)' | jq 'del(.metadata.namespace)' | jq 'del(.metadata.selfLink)' | jq 'del(.metadata.uid)' | jq 'del(.metadata.resourceVersion)' | kubectl apply --namespace=cattle-monitoring-system -f -
kubectl apply -f ./alertmanager2es.yaml
