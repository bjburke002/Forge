package ecs

import (
	"testing"

	"github.com/bjburke002/forge/pkg/contract/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func resources(cpu, memLimit string) v1alpha1.ResourceSpec {
	return v1alpha1.ResourceSpec{
		Requests: v1alpha1.ResourceRequests{
			CPU:    resource.MustParse(cpu),
			Memory: resource.MustParse(memLimit),
		},
		Limits: v1alpha1.ResourceLimits{Memory: resource.MustParse(memLimit)},
	}
}

func TestTaskSize(t *testing.T) {
	tests := []struct {
		name             string
		cpu, memLimit    string
		wantCPU, wantMem string
		wantErr          bool
	}{
		{name: "payments", cpu: "25m", memLimit: "128Mi", wantCPU: "256", wantMem: "1024"},
		{name: "exactly 0.25 vCPU", cpu: "250m", memLimit: "1Gi", wantCPU: "256", wantMem: "1024"},
		{name: "just over 0.25 vCPU", cpu: "300m", memLimit: "512Mi", wantCPU: "512", wantMem: "1024"},
		{name: "memory above the floor", cpu: "25m", memLimit: "2Gi", wantCPU: "256", wantMem: "2048"},
		{name: "memory rounds up to the next allowed value", cpu: "25m", memLimit: "1500Mi", wantCPU: "256", wantMem: "2048"},
		{name: "memory forces a larger CPU", cpu: "25m", memLimit: "3Gi", wantCPU: "512", wantMem: "3072"},
		{name: "1 vCPU", cpu: "1", memLimit: "2Gi", wantCPU: "1024", wantMem: "2048"},
		{name: "largest size", cpu: "4", memLimit: "30Gi", wantCPU: "4096", wantMem: "30720"},
		{name: "CPU too large", cpu: "5", memLimit: "1Gi", wantErr: true},
		{name: "memory too large", cpu: "1", memLimit: "31Gi", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu, mem, err := taskSize(resources(tt.cpu, tt.memLimit))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %s / %s, want an error", cpu, mem)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cpu != tt.wantCPU || mem != tt.wantMem {
				t.Errorf("got %s / %s, want %s / %s", cpu, mem, tt.wantCPU, tt.wantMem)
			}
		})
	}
}

// Every size the table can produce must be one Fargate accepts: memory ascending and
// inside AWS's documented range for that CPU.
func TestFargateSizesTable(t *testing.T) {
	ranges := map[int][2]int{256: {512, 2048}, 512: {1024, 4096}, 1024: {2048, 8192}, 2048: {4096, 16384}, 4096: {8192, 30720}}
	for _, s := range fargateSizes {
		r, ok := ranges[s.cpu]
		if !ok {
			t.Errorf("unexpected CPU size %d", s.cpu)
			continue
		}
		if first, last := s.memory[0], s.memory[len(s.memory)-1]; first != r[0] || last != r[1] {
			t.Errorf("CPU %d: memory %d–%d, want %d–%d", s.cpu, first, last, r[0], r[1])
		}
		for i := 1; i < len(s.memory); i++ {
			if s.memory[i] <= s.memory[i-1] {
				t.Errorf("CPU %d: memory not ascending at %d", s.cpu, s.memory[i])
			}
		}
	}
}
