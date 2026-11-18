#!/usr/bin/env bash
# Delete every ECS service stack before terraform destroy. 
# Service stacks are found by their tags (Project=forge, Backend=ecs)
set -euo pipefail

REGION="${1:?usage: $0 <region>}"

stacks=$(aws cloudformation describe-stacks --region "$REGION" --output text \
  --query "Stacks[?Tags[?Key=='Project' && Value=='forge'] && Tags[?Key=='Backend' && Value=='ecs']].StackName")

if [[ -z "$stacks" ]]; then
  echo "No ECS service stacks to delete."
  exit 0
fi

# Start every delete first so they run in parallel, then wait for each.
for s in $stacks; do
  echo "Deleting stack $s"
  aws cloudformation delete-stack --region "$REGION" --stack-name "$s"
done
for s in $stacks; do
  aws cloudformation wait stack-delete-complete --region "$REGION" --stack-name "$s" || {
    echo "Stack $s did not delete; check its events before running terraform destroy." >&2
    exit 1
  }
  echo "Deleted $s"
done
