#!/usr/bin/env bash
# Report AWS resources that Kubernetes controllers created and that outlived the cluster.
# EBS volumes don't block terraform destroy, so they'd keep billing unnoticed. Karpenter
# nodes, ALBs and their security groups do block it (the VPC can't be deleted), so also
# run this by hand when a destroy hangs on the VPC, to see what's holding it.
set -euo pipefail

REGION="${1:?usage: $0 <region> <cluster-name>}"
CLUSTER="${2:?usage: $0 <region> <cluster-name>}"
ec2() { aws ec2 --region "$REGION" "$@" --output text; }
found=0
report() { [[ -n "$2" ]] && { echo "Leftover $1: $2"; found=1; } || true; }

report "EBS volumes (still billing)" "$(ec2 describe-volumes \
  --filters Name=tag-key,Values=ebs.csi.aws.com/cluster --query 'Volumes[].VolumeId')"

report "Karpenter instances" "$(ec2 describe-instances \
  --filters Name=tag-key,Values=karpenter.sh/nodepool \
            Name=instance-state-name,Values=pending,running,stopping,stopped \
  --query 'Reservations[].Instances[].InstanceId')"

report "LB Controller security groups" "$(ec2 describe-security-groups \
  --filters Name=tag:elbv2.k8s.aws/cluster,Values="$CLUSTER" --query 'SecurityGroups[].GroupId')"

report "EKS cluster security group (blocks VPC deletion)" "$(ec2 describe-security-groups \
  --filters Name=tag:aws:eks:cluster-name,Values="$CLUSTER" --query 'SecurityGroups[].GroupId')"
  
# The tagging API can briefly list load balancers that were just deleted, so confirm each one.
lbs=""
for arn in $(aws resourcegroupstaggingapi get-resources --region "$REGION" \
    --tag-filters Key=elbv2.k8s.aws/cluster,Values="$CLUSTER" \
    --resource-type-filters elasticloadbalancing:loadbalancer \
    --query 'ResourceTagMappingList[].ResourceARN' --output text); do
  aws elbv2 describe-load-balancers --region "$REGION" --load-balancer-arns "$arn" >/dev/null 2>&1 \
    && lbs="$lbs ${arn##*:loadbalancer/}"
done
report "load balancers" "${lbs# }"

[[ $found -eq 0 ]] && echo "No leftovers." || exit 1
