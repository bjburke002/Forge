#!/usr/bin/env bash
# Fail if the EBS CSI driver left volumes behind. Unlike load balancers or nodes,
# they don't block terraform destroy, so they'd keep billing unnoticed.
set -euo pipefail

REGION="${1:?usage: $0 <region>}"

volumes=$(aws ec2 describe-volumes --region "$REGION" \
  --filters Name=tag-key,Values=ebs.csi.aws.com/cluster \
  --query 'Volumes[].VolumeId' --output text)

if [[ -n "$volumes" ]]; then
  echo "Leftover EBS volumes (still billing): $volumes"
  exit 1
fi
echo "No leftover volumes."
