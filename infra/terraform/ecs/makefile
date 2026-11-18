SHELL       := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

CLUSTER_NAME ?= forge-ecs
AWS_REGION   ?= us-east-1
PLAN_FILE    := tfplan

.DEFAULT_GOAL := help
.PHONY: help init fmt validate plan up down leftovers

help: ## Show available targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

init: ## terraform init
	terraform init -input=false

fmt: ## Format all Terraform files
	terraform fmt -recursive

validate: init ## Check formatting and validate configuration
	terraform fmt -check -recursive
	terraform validate

plan: validate ## Plan changes to the ECS root
	terraform plan -input=false -out=$(PLAN_FILE)

# Redeploying every ECS service from forge-gitops is added here in E4.4.
up: plan ## Apply the plan
	terraform apply -input=false $(PLAN_FILE)
	rm -f $(PLAN_FILE)

down: ## Delete the ECS service stacks, destroy the ECS root, then check for leftovers
	./scripts/ecs-teardown.sh "$(AWS_REGION)"
	terraform destroy -input=false
	./scripts/check-leftovers.sh "$(AWS_REGION)" "$(CLUSTER_NAME)"

leftovers: ## Report ECS resources that outlived a destroy
	./scripts/check-leftovers.sh "$(AWS_REGION)" "$(CLUSTER_NAME)"
