package ecs

import (
	"fmt"
	"strconv"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
)

// minTaskMemoryMiB reserves room for the ADOT collector sidecar
const minTaskMemoryMiB = 1024

// fargateSize is one Fargate CPU size and the task memory values it allows.
type fargateSize struct {
	cpu    int   // 1024 per vCPU
	memory []int // MiB, ascending
}

// fargateSizes lists the Fargate task sizes Forge renders
var fargateSizes = []fargateSize{
	{cpu: 256, memory: []int{512, 1024, 2048}},
	{cpu: 512, memory: steps(1024, 4096, 1024)},
	{cpu: 1024, memory: steps(2048, 8192, 1024)},
	{cpu: 2048, memory: steps(4096, 16384, 1024)},
	{cpu: 4096, memory: steps(8192, 30720, 1024)},
}

func steps(lo, hi, step int) []int {
	s := make([]int, 0, (hi-lo)/step+1)
	for v := lo; v <= hi; v += step {
		s = append(s, v)
	}
	return s
}

// taskSize picks the smallest Fargate task size that fits the contract
func taskSize(r v1alpha1.ResourceSpec) (cpu, memory string, err error) {
	// Round up: a task must never get less than the contract asks for.
	cpuUnits := int((r.Requests.CPU.MilliValue()*1024 + 999) / 1000)
	const mib = 1 << 20
	memMiB := int((r.Limits.Memory.Value() + mib - 1) / mib)
	memMiB = max(memMiB, minTaskMemoryMiB)

	for _, size := range fargateSizes {
		if size.cpu < cpuUnits {
			continue
		}
		for _, m := range size.memory {
			if m >= memMiB {
				return strconv.Itoa(size.cpu), strconv.Itoa(m), nil
			}
		}
	}
	return "", "", fmt.Errorf("no Fargate task size fits a CPU request of %s and a memory limit of %s (largest is 4 vCPU and 30 GB)",
		r.Requests.CPU.String(), r.Limits.Memory.String())
}
