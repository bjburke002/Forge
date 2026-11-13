// Package crd is responsible for embedding the ForgeService CRD manifest so that it can be utilized in the CLI
package crd

import (
	_ "embed"
)

// ForgeService is the generated ForgeService CRD manifest, generated at runtime
//
//go:embed forge.dev_forgeservices.yaml
var ForgeService []byte
