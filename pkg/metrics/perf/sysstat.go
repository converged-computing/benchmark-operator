package perf

import (
	"fmt"

	api "github.com/converged-computing/metrics-operator/api/v1alpha1"

	metrics "github.com/converged-computing/metrics-operator/pkg/metrics"
)

// sysstat provides a tool "pidstat" that can monitor a PID (along with others)
// https://github.com/sysstat/sysstat

type PidStat struct {
	name                string
	rate                int32
	description         string
	container           string
	requiresApplication bool
}

// Name returns the metric name
func (m PidStat) Name() string {
	return m.name
}

// Description returns the metric description
func (m PidStat) Description() string {
	return m.description
}

// Container
func (m PidStat) Image() string {
	return m.container
}

// WorkingDir does not matter
func (m PidStat) WorkingDir() string {
	return ""
}

// Set custom options / attributes for the metric
func (m PidStat) SetOptions(metric *api.Metric) {
	m.rate = metric.Rate
}

// Generate the replicated job for measuring the application
// We provide the entire Metrics Set (including the application) if we need
// to extract metadata from elsewhere
// TODO need to think of more clever way to export the values?
// Save to somewhere?
func (m PidStat) EntrypointScript(set *api.MetricSet) string {

	template := `#!/bin/bash

# Download the wait binary
wget https://github.com/converged-computing/goshare/releases/download/2023-07-27/wait
echo "Waiting for application PID..."
pid=$(wait -c "%s" -q)

i=0
while true
  do
    echo "CPU STATISTICS TIMEPOINT ${i}
    pidstat -p ${pid} -u -h
    echo "KERNEL STATISTICS TIMEPOINT ${i}
    pidstat -p ${pid} -d -h
    echo "POLICY TIMEPOINT ${i}
    pidstat -p ${pid} -R -h
    echo "PAGEFAULTS and MEMORY ${i}
	pidstat -p 30 -r -h
    echo "STACK UTILIZATION ${i}
	pidstat -p 30 -s -h
    echo "THREADS ${i}	
	pidstat -p 30 -t -h
    echo "KERNEL TABLES ${i}	
	34  pidstat -p 30 -v -h
    echo "TASK SWITCHING ${i}	
	35  pidstat -p 30 -w -h
	sleep %d
	let i=i+1 
done
`
	// NOTE: the entrypoint is the entrypoint for the container, while
	// the command is expected to be what we are monitoring. Often
	// they are the same thing.
	return fmt.Sprintf(template, set.Spec.Application.Command, m.rate)
}

// ghcr.io/converged-computing/benchmark-sysstat:latest

// Does the metric require an application container?
func (m PidStat) RequiresApplication() bool {
	return m.requiresApplication
}

func init() {
	metrics.Register(PidStat{
		name:                "perf-sysstat",
		description:         "statistics for Linux tasks (processes) : I/O, CPU, memory, etc.",
		requiresApplication: true,
		container:           "ghcr.io/converged-computing/benchmark-sysstat:latest",
	})
}
