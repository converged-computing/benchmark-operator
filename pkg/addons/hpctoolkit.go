/*
Copyright 2023 Lawrence Livermore National Security, LLC
 (c.f. AUTHORS, NOTICE.LLNS, COPYING)

SPDX-License-Identifier: MIT
*/

package addons

import (
	"fmt"
	"path/filepath"

	api "github.com/converged-computing/metrics-operator/api/v1alpha1"
	"github.com/converged-computing/metrics-operator/pkg/metadata"
	"github.com/converged-computing/metrics-operator/pkg/specs"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	jobset "sigs.k8s.io/jobset/api/jobset/v1alpha2"
)

// HPCToolkit is an addon that provides a container that

type HPCToolkit struct {
	ApplicationAddon

	// Target is the name of the replicated job to customize entrypoint logic for
	target         string
	events         string
	mount          string
	entrypointPath string
}

// AssembleVolumes to provide an empty volume for the application to share
func (m HPCToolkit) AssembleVolumes() specs.VolumeSpec {
	volume := corev1.Volume{
		Name: "hpctoolkit",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	// EmptyDir should be ReadOnly False, and we don't need a mount for it
	return specs.VolumeSpec{
		Volume: volume,
		Mount:  false,
	}
}

// Validate we have an executable provided, and args and optional
func (a *HPCToolkit) Validate() bool {
	if a.events == "" {
		logger.Error("The HPCtoolkit application addon requires one or more 'events' for hpcrun (e.g., -e IO).")
		return false
	}
	if a.image == "" {
		logger.Error("The application addon requires a container 'image'.")
		return false
	}
	if a.command == "" {
		logger.Error("The application addon requires a container 'command'.")
		return false
	}
	return true
}

// Set custom options / attributes for the metric
func (a *HPCToolkit) SetOptions(metric *api.MetricAddon) {

	a.entrypointPath = "/metrics_operator/hpctoolkit-entrypoint.sh"
	a.image = "ghcr.io/converged-computing/metric-hpctoolkit-view:latest"
	a.SetDefaultOptions(metric)
	a.mount = "/opt/share"

	// UseColor set to anything means to use it
	mount, ok := metric.Options["mount"]
	if ok {
		a.mount = mount.StrVal
	}
	events, ok := metric.Options["events"]
	if ok {
		a.events = events.StrVal
	}
}

// Exported options and list options
func (a HPCToolkit) Options() map[string]intstr.IntOrString {
	options := a.DefaultOptions()
	options["events"] = intstr.FromString(a.events)
	options["mount"] = intstr.FromString(a.mount)
	return options
}

// CustomizeEntrypoint scripts
func (a *HPCToolkit) CustomizeEntrypoints(
	cs []*specs.ContainerSpec,
	rjs []*jobset.ReplicatedJob,
) {
	for _, rj := range rjs {

		// Only customize if the replicated job name matches the target
		if a.target != "" && a.target != rj.Name {
			continue
		}
		a.customizeEntrypoint(cs, rj)
	}

}

// CustomizeEntrypoint for a single replicated job
func (a *HPCToolkit) customizeEntrypoint(
	cs []*specs.ContainerSpec,
	rj *jobset.ReplicatedJob,
) {

	// Generate addon metadata
	meta := Metadata(a)

	// This should be run after the pre block of the script
	preBlock := `
echo "%s"
# Ensure hpcrun and software exists. This is rough, but should be OK with enough wait time
wget https://github.com/converged-computing/goshare/releases/download/2023-09-06/wait-fs
chmod +x ./wait-fs
mv ./wait-fs /usr/bin/goshare-wait-fs
	
# Ensure spack view is on the path, wherever it is mounted
viewbase="%s"
software="${viewbase}/software"
viewbin="${viewbase}/view/bin"
export PATH=${viewbin}:$PATH
	
# Wait for software directory, and give it time
goshare-wait-fs -p ${software}
	
# Wait for copy to finish
sleep 10
	
# Copy mount software to /opt/software
cp -R %s/software /opt/software
	
# Wait for hpcrun
goshare-wait-fs -p ${viewbin}/hpcrun
	
# This will work with capability SYS_ADMIN added.
# It will only work with privileged set to true AT YOUR OWN RISK!
echo "-1" | tee /proc/sys/kernel/perf_event_paranoid
	
# Run hpcrun. See options with hpcrun -L
events="%s"
echo "%s"
echo "%s"
	
# Commands to interact with output data
# hpcprof hpctoolkit-sleep-measurements
# hpcstruct hpctoolkit-sleep-measurements
# hpcviewer ./hpctoolkit-lmp-database
`
	preBlock = fmt.Sprintf(
		meta,
		preBlock,
		a.mount,
		a.mount,
		a.events,
		metadata.CollectionStart,
		metadata.Separator,
	)

	// TODO we may want to target specific entrypoint scripts here
	// Right now we target all scripts associated with the job
	for _, containerSpec := range cs {

		// First check - is this the right replicated job?
		if containerSpec.JobName != rj.Name {
			continue
		}
		containerSpec.EntrypointScript.Pre = "\n" + preBlock
		containerSpec.EntrypointScript.Command = fmt.Sprintf("hpcrun $events %s", containerSpec.EntrypointScript.Command)
	}
}

// Generate a container spec that will map to a listing of containers for the replicated job
func (a *HPCToolkit) AssembleContainers() []specs.ContainerSpec {

	// The entrypoint script
	// This is the addon container entrypoint, we don't care about metadata here
	// The sole purpose is just to provide the volume, meaning copying content there
	template := `#!/bin/bash

echo "Moving content from /opt/view to be in shared volume at %s"
view=$(ls /opt/views/._view/)
view="/opt/views/._view/${view}"

# Give a little extra wait time
sleep 10

viewroot="%s"
mkdir -p $viewroot/view
# We have to move both of these paths, *sigh*
cp -R ${view}/* $viewroot/view
cp -R /opt/software $viewroot/

# Sleep forever, the application needs to run and end
echo "Sleeping forever so %s can be shared and use for hpctoolkit."
sleep infinity
`
	script := fmt.Sprintf(
		template,
		a.mount,
		a.mount,
		a.mount,
	)

	// Leave the name empty to generate in the namespace of the metric set (e.g., set.Name)
	entrypoint := specs.EntrypointScript{
		Path:   a.entrypointPath,
		Script: filepath.Base(a.entrypointPath),
		Pre:    script,
	}

	// The resource spec and attributes for now are empty (might redo this design)
	// We assume they inherit the resources / attributes of the pod for now
	// We don't use JobName here because we don't associate addon containers
	// with other addon entrypoints
	return []specs.ContainerSpec{
		{
			Image:            a.image,
			Name:             "hpctoolkit",
			EntrypointScript: entrypoint,
			Resources:        &api.ContainerResources{},
			Attributes: &api.ContainerSpec{
				SecurityContext: api.SecurityContext{
					Privileged: a.privileged,
				},
			},
		},
	}
}

func init() {
	base := AddonBase{
		Identifier: "perf-hpctoolkit",
		Summary:    "performance tools for measurement and analysis",
	}
	app := ApplicationAddon{AddonBase: base}
	HPCToolkit := HPCToolkit{ApplicationAddon: app}
	Register(&HPCToolkit)
}
