/*
Copyright 2023 Lawrence Livermore National Security, LLC
 (c.f. AUTHORS, NOTICE.LLNS, COPYING)

SPDX-License-Identifier: MIT
*/

package addons

import (
	"fmt"
	"path/filepath"

	api "github.com/converged-computing/metrics-operator/api/v1alpha2"
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
	target string

	// ContainerTarget is the name of the container to add the entrypoint logic to
	containerTarget string
	events          string
	mount           string
	entrypointPath  string
	volumeName      string

	// For mpirun and similar, mpirun needs to wrap hpcrun and the command, e.g.,
	// mpirun <MPI args> hpcrun <hpcrun args> <app> <app args>
	prefix string
}

func (m HPCToolkit) Family() string {
	return AddonFamilyPerformance
}

// AssembleVolumes to provide an empty volume for the application to share
// We also need to provide a config map volume for our container spec
func (m HPCToolkit) AssembleVolumes() []specs.VolumeSpec {
	volume := corev1.Volume{
		Name: m.volumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}

	// Prepare items as key to path
	items := []corev1.KeyToPath{
		{
			Key:  m.volumeName,
			Path: filepath.Base(m.entrypointPath),
		},
	}

	// This is a config map volume with items
	// It needs to be created in the same metrics operator namespace
	// Thus we only need the items!
	configVolume := corev1.Volume{
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				Items: items,
			},
		},
	}

	// EmptyDir should be ReadOnly False, and we don't need a mount for it
	return []specs.VolumeSpec{
		{
			Volume: volume,
			Mount:  true,
			Path:   m.mount,
		},

		// Mount is set to false here because we mount via metrics_operator
		{
			Volume:   configVolume,
			ReadOnly: true,
			Mount:    false,
			Path:     filepath.Dir(m.entrypointPath),
		},
	}
}

// Validate we have an executable provided, and args and optional
func (a *HPCToolkit) Validate() bool {
	if a.events == "" {
		logger.Error("The HPCtoolkit application addon requires one or more 'events' for hpcrun (e.g., -e IO).")
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
	a.volumeName = "hpctoolkit"

	// UseColor set to anything means to use it
	mount, ok := metric.Options["mount"]
	if ok {
		a.mount = mount.StrVal
	}
	prefix, ok := metric.Options["prefix"]
	if ok {
		a.prefix = prefix.StrVal
	}
	workdir, ok := metric.Options["workdir"]
	if ok {
		a.workdir = workdir.StrVal
	}
	target, ok := metric.Options["target"]
	if ok {
		a.target = target.StrVal
	}
	ctarget, ok := metric.Options["containerTarget"]
	if ok {
		a.containerTarget = ctarget.StrVal
	}
	events, ok := metric.Options["events"]
	if ok {
		a.events = events.StrVal
	}
}

// Exported options and list options
func (a *HPCToolkit) Options() map[string]intstr.IntOrString {
	options := a.DefaultOptions()
	options["events"] = intstr.FromString(a.events)
	options["mount"] = intstr.FromString(a.mount)
	options["prefix"] = intstr.FromString(a.prefix)
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
hpcrunpath=${viewbin}/hpcrun

# Important to add AFTER in case software in container duplicated
export PATH=$PATH:${viewbin}
	
# Wait for software directory, and give it time
goshare-wait-fs -p ${software}
	
# Wait for copy to finish
sleep 10
	
# Copy mount software to /opt/software
cp -R %s/software /opt/software
	
# Wait for hpcrun and marker to indicate copy is done
goshare-wait-fs -p ${viewbin}/hpcrun
goshare-wait-fs -p ${viewbase}/metrics-operator-done.txt

# A small extra wait time to be conservative
sleep 5

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
		preBlock,
		meta,
		a.mount,
		a.mount,
		a.events,
		metadata.CollectionStart,
		metadata.Separator,
	)

	// Add the working directory, if defined
	if a.workdir != "" {
		preBlock += fmt.Sprintf(`
workdir="%s"
echo "Changing directory to ${workdir}"
cd ${workdir}			
`, a.workdir)
	}

	// We use container names to target specific entrypoint scripts here
	for _, containerSpec := range cs {

		// First check - is this the right replicated job?
		if containerSpec.JobName != rj.Name {
			continue
		}

		// Always copy over the pre block - we need the logic to copy software
		containerSpec.EntrypointScript.Pre += "\n" + preBlock

		// Next check if we have a target set (for the container)
		if a.containerTarget != "" && containerSpec.Name != "" && a.containerTarget != containerSpec.Name {
			continue
		}
		containerSpec.EntrypointScript.Command = fmt.Sprintf("%s $hpcrunpath $events %s", a.prefix, containerSpec.EntrypointScript.Command)
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

# This is a marker to indicate the copy is done
touch $viewroot/metrics-operator-done.txt

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
		Name:   a.volumeName,
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
			// We need to write this config map!
			NeedsWrite: true,
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
