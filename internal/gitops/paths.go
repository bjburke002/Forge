package gitops

import (
	"path"
)

func ServicePath(target, svcName, overlay, fileName string) string {
	return path.Join(target, "services", svcName, overlay, fileName)
}
