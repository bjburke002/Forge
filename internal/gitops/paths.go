package gitops

import (
	"path"
)

const (
	TargetEKS = "eks"
	TargetECS = "ecs"
)

func ServicePath(target, svcName, overlay, fileName string) string {
	return path.Join(target, "services", svcName, overlay, fileName)
}
