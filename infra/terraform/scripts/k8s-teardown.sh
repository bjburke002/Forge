#!/usr/bin/env bash
# Delete Kubernetes resources that create AWS infrastructure outside Terraform
# (load balancers, EBS volumes, Karpenter nodes) so terraform destroy can remove the VPC.
set -euo pipefail

USAGE="usage: $0 <cluster-name> <region> <domain>"
CLUSTER_NAME="${1:?$USAGE}"
REGION="${2:?$USAGE}"
DOMAIN="${3:?$USAGE}"
TXT_OWNER="forge" # must match txtOwnerId in charts/platform/external-dns.yaml

# Nothing to clean up if the cluster is already gone (e.g. re-running after a failed destroy).
CTX=$(aws eks describe-cluster --name "$CLUSTER_NAME" --region "$REGION" \
  --query cluster.arn --output text 2>/dev/null) || {
  echo "Cluster $CLUSTER_NAME not found; skipping Kubernetes cleanup."
  exit 0
}
aws eks update-kubeconfig --name "$CLUSTER_NAME" --region "$REGION" >/dev/null

# Always target this cluster explicitly, never whatever context happens to be current.
kc() { kubectl --context "$CTX" "$@"; }

# Stop Argo CD re-creating what we delete (no-op until Argo is installed).
kc -n argocd scale statefulset argocd-application-controller --replicas=0 2>/dev/null || true

# Each delete waits on the owning controller's finalizer, i.e. until the AWS resource is gone.
kc delete ingress --all -A --timeout=10m
kc get svc -A -o json |
  jq -r '.items[] | select(.spec.type == "LoadBalancer") | "\(.metadata.namespace) \(.metadata.name)"' |
  while read -r ns name; do kc -n "$ns" delete svc "$name" --timeout=10m; done

# Give ExternalDNS time to delete the records for those load balancers while it still
# has a node and credentials. It removes each record together with its TXT ownership
# record, so none of those left means it has finished.
ZONE_ID=$(aws route53 list-hosted-zones-by-name --dns-name "$DOMAIN" --max-items 1 \
  --query "HostedZones[?Name=='${DOMAIN}.'].Id | [0]" --output text)
owned_records() {
  aws route53 list-resource-record-sets --hosted-zone-id "$ZONE_ID" --output text \
    --query "length(ResourceRecordSets[?Type=='TXT' && contains(join('', ResourceRecords[].Value), 'external-dns/owner=${TXT_OWNER}')])"
}
deadline=$((SECONDS + 180))
while (( $(owned_records) > 0 )); do
  if (( SECONDS >= deadline )); then
    echo "Warning: ExternalDNS records remain in $DOMAIN; the next make up will adopt them."
    break
  fi
  sleep 10
done

#Accounting for pods with attached PVCs
if [[ -n "$(kc get pvc -A -o name)" ]]; then
  # Mark PVCs for deletion first, so replacement pods can't mount them again
  kc delete pvc --all -A --wait=false
  # Remove only the pods that use a PVC
  kc get pods -A -o json |
    jq -r '.items[] | select(any(.spec.volumes[]?; .persistentVolumeClaim)) | "\(.metadata.namespace) \(.metadata.name)"' |
    while read -r ns name; do kc -n "$ns" delete pod "$name" --timeout=10m; done
  # Wait until the PVCs (and their EBS volumes) are actually gone
  kc wait --for=delete pvc --all -A --timeout=10m
fi

kc delete pdb --all -A

if kc get crd nodepools.karpenter.sh >/dev/null 2>&1; then
  kc delete nodepools --all --timeout=10m
fi

echo "Kubernetes cleanup complete."
