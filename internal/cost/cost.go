package cost

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	DefaultCPUCostPerVCPUHour  = 0.031
	DefaultMemoryCostPerGBHour = 0.004
	DefaultGPUCostPerHour      = 2.50
)

type Report struct {
	Model                string
	Replicas             int32
	CPU                  float64
	MemoryGB             float64
	GPU                  int64
	EstimatedHourlyCost  float64
	EstimatedDailyCost   float64
	EstimatedMonthlyCost float64
}

func Estimate(modelName, cpu, memory string, gpu int64, replicas int32) (Report, error) {
	if replicas <= 0 {
		replicas = 1
	}

	var cpuValue float64
	if cpu != "" {
		q, err := resource.ParseQuantity(cpu)
		if err != nil {
			return Report{}, fmt.Errorf("parse cpu quantity: %w", err)
		}
		cpuValue = float64(q.MilliValue()) / 1000
	}

	var memoryGB float64
	if memory != "" {
		q, err := resource.ParseQuantity(memory)
		if err != nil {
			return Report{}, fmt.Errorf("parse memory quantity: %w", err)
		}
		memoryGB = float64(q.Value()) / (1024 * 1024 * 1024)
	}

	hourlyPerReplica := cpuValue*DefaultCPUCostPerVCPUHour + memoryGB*DefaultMemoryCostPerGBHour + float64(gpu)*DefaultGPUCostPerHour
	hourly := hourlyPerReplica * float64(replicas)

	return Report{
		Model:                modelName,
		Replicas:             replicas,
		CPU:                  cpuValue * float64(replicas),
		MemoryGB:             memoryGB * float64(replicas),
		GPU:                  gpu * int64(replicas),
		EstimatedHourlyCost:  hourly,
		EstimatedDailyCost:   hourly * 24,
		EstimatedMonthlyCost: hourly * 730,
	}, nil
}

func (r Report) YAML() string {
	return fmt.Sprintf(
		"model: %s\nreplicas: %d\ncpu: %.3f\nmemoryGB: %.3f\ngpu: %d\nestimatedHourlyCost: \"$%.3f\"\nestimatedDailyCost: \"$%.2f\"\nestimatedMonthlyCost: \"$%.2f\"\npricing:\n  cpuPerVCPUHour: \"$%.3f\"\n  memoryPerGBHour: \"$%.3f\"\n  gpuPerHour: \"$%.2f\"\nnote: Pricing is an estimate and varies by cloud provider.\n",
		r.Model,
		r.Replicas,
		r.CPU,
		r.MemoryGB,
		r.GPU,
		r.EstimatedHourlyCost,
		r.EstimatedDailyCost,
		r.EstimatedMonthlyCost,
		DefaultCPUCostPerVCPUHour,
		DefaultMemoryCostPerGBHour,
		DefaultGPUCostPerHour,
	)
}
