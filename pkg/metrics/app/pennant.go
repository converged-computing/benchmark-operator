/*
Copyright 2023 Lawrence Livermore National Security, LLC
 (c.f. AUTHORS, NOTICE.LLNS, COPYING)

SPDX-License-Identifier: MIT
*/

package application

import (
	"fmt"

	api "github.com/converged-computing/metrics-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/converged-computing/metrics-operator/pkg/jobs"
	metrics "github.com/converged-computing/metrics-operator/pkg/metrics"
)

type Pennant struct {
	jobs.LauncherWorker

	// Custom Options
	command string
	prefix  string
}

// I think this is a simulation?
func (m Pennant) Family() string {
	return metrics.SimulationFamily
}

func (m Pennant) Url() string {
	return "https://github.com/LLNL/pennant"
}

// Set custom options / attributes for the metric
func (m *Pennant) SetOptions(metric *api.Metric) {
	m.ResourceSpec = &metric.Resources
	m.AttributeSpec = &metric.Attributes

	// Set user defined values or fall back to defaults
	m.prefix = "mpirun --hostfile ./hostlist.txt"
	m.command = "pennant /opt/pennant/test/sedovsmall/sedovsmall.pnt"
	m.Workdir = "/opt/pennant/test"

	// This could be improved :)
	command, ok := metric.Options["command"]
	if ok {
		m.command = command.StrVal
	}
	workdir, ok := metric.Options["workdir"]
	if ok {
		m.Workdir = workdir.StrVal
	}
	mpirun, ok := metric.Options["mpirun"]
	if ok {
		m.prefix = mpirun.StrVal
	}
}

// Exported options and list options
func (m Pennant) Options() map[string]intstr.IntOrString {
	return map[string]intstr.IntOrString{
		"command": intstr.FromString(m.command),
		"mpirun":  intstr.FromString(m.prefix),
		"workdir": intstr.FromString(m.Workdir),
	}
}
func (n Pennant) ListOptions() map[string][]intstr.IntOrString {
	return map[string][]intstr.IntOrString{}
}

// Return lookup of entrypoint scripts
func (m Pennant) EntrypointScripts(
	spec *api.MetricSet,
	metric *metrics.Metric,
) []metrics.EntrypointScript {

	// Metadata to add to beginning of run
	metadata := metrics.Metadata(spec, metric)
	hosts := m.GetHostlist(spec)
	prefix := m.GetCommonPrefix(metadata, m.command, hosts)

	// Template for the launcher
	template := `
echo "%s"
%s ./problem.sh
echo "%s"
%s
`
	launcherTemplate := prefix + fmt.Sprintf(
		template,
		metrics.Separator,
		m.prefix,
		metrics.CollectionEnd,
		metrics.Interactive(spec.Spec.Logging.Interactive),
	)

	// The worker just has sleep infinity added
	workerTemplate := prefix + "\nsleep infinity"
	return m.FinalizeEntrypoints(launcherTemplate, workerTemplate)
}

func init() {
	launcher := jobs.LauncherWorker{
		Identifier:     "app-pennant",
		Summary:        "Unstructured mesh hydrodynamics for advanced architectures ",
		Container:      "ghcr.io/converged-computing/metric-pennant:latest",
		WorkerScript:   "/metrics_operator/pennant-worker.sh",
		LauncherScript: "/metrics_operator/pennant-launcher.sh",
	}
	Pennant := Pennant{LauncherWorker: launcher}
	metrics.Register(&Pennant)
}