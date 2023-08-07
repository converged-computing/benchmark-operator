/*
Copyright 2022-2023 Lawrence Livermore National Security, LLC
 (c.f. AUTHORS, NOTICE.LLNS, COPYING)

SPDX-License-Identifier: MIT
*/

package metrics

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"

	api "github.com/converged-computing/metrics-operator/api/v1alpha1"
)

// A ContainerSpec is used by a metric to define a container
type ContainerSpec struct {
	Command    []string
	Image      string
	Name       string
	WorkingDir string
}

// Named entrypoint script for a container
type EntrypointScript struct {
	Name   string
	Path   string
	Script string
}

// getContainers gets containers for a set of metrics
func getContainers(
	set *api.MetricSet,
	metrics []*Metric,
	volumes map[string]api.Volume,
) ([]corev1.Container, error) {

	containers := []ContainerSpec{}

	// Create one container per metric!
	// Each needs to have the sys trace capability to see the application pids
	for i, m := range metrics {

		script := fmt.Sprintf("/metrics_operator/entrypoint-%d.sh", i)
		command := []string{"/bin/bash", script}

		newContainer := ContainerSpec{
			Command:    command,
			Image:      (*m).Image(),
			WorkingDir: (*m).WorkingDir(),
			Name:       (*m).Name(),
		}
		containers = append(containers, newContainer)
	}
	return GetContainers(set, containers, volumes, false)
}

// GetContainers based on one or more container specs
func GetContainers(
	set *api.MetricSet,
	specs []ContainerSpec,
	volumes map[string]api.Volume,
	allowPtrace bool,
) ([]corev1.Container, error) {

	// Assume we can pull once for now, this could be changed to allow
	// corev2.PullAlways
	pullPolicy := corev1.PullIfNotPresent
	containers := []corev1.Container{}

	// Currently we share the same mounts across containers, makes life easier!
	mounts := getVolumeMounts(set, volumes)

	// Create one container per metric!
	// Each needs to have the sys trace capability to see the application pids
	for _, s := range specs {

		// TODO specify container resources here?

		// Assemble the container for the node
		// Name the container by the metric for now
		newContainer := corev1.Container{
			Name:            s.Name,
			Image:           s.Image,
			ImagePullPolicy: pullPolicy,
			VolumeMounts:    mounts,
			Stdin:           true,
			TTY:             true,
			Command:         s.Command,
		}

		// Should we allow sharing the process namespace?
		if allowPtrace {
			newContainer.SecurityContext = &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"SYS_PTRACE"},
				},
			}
		}

		// Only add the working directory if it's defined
		if s.WorkingDir != "" {
			newContainer.WorkingDir = s.WorkingDir
		}

		// Ports and environment
		// TODO this should be added when needed
		ports := []corev1.ContainerPort{}
		envars := []corev1.EnvVar{}
		newContainer.Ports = ports
		newContainer.Env = envars
		containers = append(containers, newContainer)
	}

	// If our metric set has an application, add it last
	if set.HasApplication() {
		command := []string{"/bin/bash", "-c", set.Spec.Application.Entrypoint}
		appContainer := corev1.Container{
			Name:            "app",
			Image:           set.Spec.Application.Image,
			ImagePullPolicy: pullPolicy,
			Stdin:           true,
			TTY:             true,
			Command:         command,
		}
		containers = append(containers, appContainer)
	}
	fmt.Printf("🟪️ Adding %d containers\n", len(containers))
	return containers, nil
}