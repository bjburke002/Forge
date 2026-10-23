SHELL       := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

CLUSTER_NAME ?= forge
AWS_REGION   ?= us-east-1
DOMAIN       ?= forgeappservices.com
PLAN_FILE    := tfplan

.DEFAULT_GOAL := help
.PHONY: help init fmt validate plan up down kubeconfig

help: ## Show available targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

init: ## terraform init
	terraform init -input=false

fmt: ## Format all Terraform files
	terraform fmt -recursive

validate: init ## Check formatting and validate configuration
	terraform fmt -check -recursive
	terraform validate

plan: validate ## Plan changes to the main root
	terraform plan -input=false -out=$(PLAN_FILE)

up: plan ## Apply the plan and configure kubectl
	terraform apply -input=false $(PLAN_FILE)
	rm -f $(PLAN_FILE)
	$(MAKE) kubeconfig
	kubectl wait --for=condition=Ready nodes --all --timeout=10m

down:
	./scripts/k8s-teardown.sh "$(CLUSTER_NAME)" "$(AWS_REGION)" "$(DOMAIN)"
	terraform destroy -input=false
	./scripts/check-leftovers.sh $(AWS_REGION)

kubeconfig: ## Point kubectl at the cluster
	aws eks update-kubeconfig --region $(AWS_REGION) --name $(CLUSTER_NAME)