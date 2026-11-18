#!/usr/bin/env bash
# Report ECS resources that outlived terraform destroy
set -euo pipefail

REGION="${1:?usage: $0 <region> <cluster-name>}"
CLUSTER="${2:?usage: $0 <region> <cluster-name>}"
found=0
report() { [[ -n "$2" ]] && { echo "Leftover $1: $2"; found=1; } || true; }

report "ECS service stacks" "$(aws cloudformation describe-stacks --region "$REGION" --output text \
  --query "Stacks[?Tags[?Key=='Project' && Value=='forge'] && Tags[?Key=='Backend' && Value=='ecs']].StackName")"

report "ECS cluster" "$(aws ecs describe-clusters --region "$REGION" --clusters "$CLUSTER" --output text \
  --query "clusters[?status=='ACTIVE'].clusterName")"

report "load balancer" "$(aws elbv2 describe-load-balancers --region "$REGION" --names "$CLUSTER-alb" \
  --output text --query 'LoadBalancers[].LoadBalancerName' 2>/dev/null || true)"

# Direct checks on what would keep billing or block a destroy. 
ec2() { aws ec2 --region "$REGION" "$@" --output text; }
TAGS=(Name=tag:Project,Values=forge Name=tag:Backend,Values=ecs)

report "NAT gateways (billing)" "$(ec2 describe-nat-gateways \
  --filter "${TAGS[@]}" Name=state,Values=pending,available \
  --query 'NatGateways[].NatGatewayId')"

report "Elastic IPs (billing)" "$(ec2 describe-addresses --filters "${TAGS[@]}" \
  --query 'Addresses[].AllocationId')"

report "VPC" "$(ec2 describe-vpcs --filters "${TAGS[@]}" --query 'Vpcs[].VpcId')"

[[ $found -eq 0 ]] && echo "No leftovers." || exit 1
