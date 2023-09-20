/*
Copyright 2023 Lawrence Livermore National Security, LLC
 (c.f. AUTHORS, NOTICE.LLNS, COPYING)

SPDX-License-Identifier: MIT
*/

package addons

import (
	"path/filepath"

	corev1 "k8s.io/api/core/v1"

	api "github.com/converged-computing/metrics-operator/api/v1alpha1"
	"github.com/converged-computing/metrics-operator/pkg/specs"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type VolumeBase struct {
	AddonBase
	readOnly bool
	name     string
	path     string
}

func (v *VolumeBase) DefaultValidate() bool {
	if v.name == "" {
		logger.Error("All volume addons require a 'name' for reference.")
		return false
	}
	if v.path == "" {
		logger.Error("All volume addons require a 'path' for the container mount.")
		return false
	}
	return true
}

// DefaultSetOptions across volume types for shared attributes
func (v *VolumeBase) DefaultSetOptions(metric *api.MetricAddon) {

	// ConfigMap names
	name, ok := metric.Options["name"]
	if ok {
		v.name = name.StrVal
	}
	path, ok := metric.Options["path"]
	if ok {
		v.path = path.StrVal
	}
	readOnly, ok := metric.Options["readOnly"]
	if ok {
		if readOnly.StrVal == "yes" || readOnly.StrVal == "true" {
			v.readOnly = true
		}
	}
}

// A general metric is a container added to a JobSet
type ConfigMapVolume struct {
	VolumeBase

	// Config map name is required for an existing config map
	// The metrics operator does not create it for you!
	configMapName string

	// name and path of the volume
	name string
	path string

	// Items (key and paths) for the config map
	items map[string]string
}

// Validate we have an executable provided, and args and optional
func (v *ConfigMapVolume) Validate() bool {
	if v.configMapName == "" {
		logger.Error("The volume-cm volume addon requires a 'configMapName' for the existing config map.")
		return false
	}
	if len(v.items) == 0 {
		logger.Error("The volume-cm volume addon requires at least one entry in mapOptions->items, with key value pairs.")
		return false
	}
	return v.DefaultValidate()
}

// Set custom options / attributes for the metric
func (v *ConfigMapVolume) SetOptions(metric *api.MetricAddon) {

	// Set an empty list of items
	v.items = map[string]string{}

	name, ok := metric.Options["configMapName"]
	if ok {
		v.configMapName = name.StrVal
	}

	// Items for the config map
	items, ok := metric.MapOptions["items"]
	if ok {
		for k, value := range items {
			v.items[k] = value.StrVal
		}
	}
	v.DefaultSetOptions(metric)
}

// Exported options and list options
func (v *ConfigMapVolume) Options() map[string]intstr.IntOrString {
	return map[string]intstr.IntOrString{
		"path":          intstr.FromString(v.path),
		"name":          intstr.FromString(v.name),
		"configMapName": intstr.FromString(v.configMapName),
	}
}

// Return formatted map options
func (v *ConfigMapVolume) MapOptions() map[string]map[string]intstr.IntOrString {
	items := map[string]intstr.IntOrString{}
	for k, value := range v.items {
		items[k] = intstr.FromString(value)
	}
	return map[string]map[string]intstr.IntOrString{
		"items": items,
	}
}

// AssembleVolumes for a config map
func (v *ConfigMapVolume) AssembleVolumes() []specs.VolumeSpec {

	// Prepare items as key to path
	items := []corev1.KeyToPath{}
	for key, path := range v.items {
		newItem := corev1.KeyToPath{
			Key:  key,
			Path: path,
		}
		items = append(items, newItem)
	}

	// This is a config map volume with items
	newVolume := corev1.Volume{
		Name: v.name,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: v.configMapName,
				},
				Items: items,
			},
		},
	}

	// ConfigMaps have to be read only!
	return []specs.VolumeSpec{{
		Volume:   newVolume,
		Path:     filepath.Dir(v.path),
		ReadOnly: true,
		Mount:    true,
	}}
}

// An existing peristent volume claim
type PersistentVolumeClaim struct {
	VolumeBase

	// Path and claim name are always required if a secret isn't defined
	name      string
	claimName string
	path      string
}

// Validate we have an executable provided, and args and optional
func (v *PersistentVolumeClaim) Validate() bool {
	if v.claimName == "" {
		logger.Error("The volume-pvc volume addon requires a 'claimName' for the existing persistent volume claim (pvc).")
		return false
	}
	return v.DefaultValidate()
}

// Set custom options / attributes
func (v *PersistentVolumeClaim) SetOptions(metric *api.MetricAddon) {
	claimName, ok := metric.Options["claimName"]
	if ok {
		v.claimName = claimName.StrVal
	}
	v.DefaultSetOptions(metric)
}

// AssembleVolumes for a pvc
func (v *PersistentVolumeClaim) AssembleVolumes() []specs.VolumeSpec {
	volume := corev1.Volume{
		Name: v.name,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: v.claimName,
			},
		},
	}

	// ConfigMaps have to be read only!
	return []specs.VolumeSpec{{
		Volume:   volume,
		Path:     filepath.Dir(v.path),
		ReadOnly: v.readOnly,
		Mount:    true,
	}}
}

// An existing secret
type SecretVolume struct {
	VolumeBase

	// secret name is required
	secretName string
	name       string
	path       string
}

// Validate we have an executable provided, and args and optional
func (v *SecretVolume) Validate() bool {
	if v.secretName == "" {
		logger.Error("The volume-secret addon requires a 'secretName' for the existing secret.")
		return false
	}
	return v.DefaultValidate()
}

// Set custom options / attributes
func (v *SecretVolume) SetOptions(metric *api.MetricAddon) {
	secretName, ok := metric.Options["secretName"]
	if ok {
		v.secretName = secretName.StrVal
	}
	v.DefaultSetOptions(metric)
}

// AssembleVolumes for a Secret
func (v *SecretVolume) AssembleVolumes() []specs.VolumeSpec {
	volume := corev1.Volume{
		Name: v.name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: v.secretName,
			},
		},
	}
	return []specs.VolumeSpec{{
		Volume:   volume,
		ReadOnly: v.readOnly,
		Path:     v.path,
		Mount:    true,
	}}
}

// A hostPath volume
type HostPathVolume struct {
	VolumeBase

	// only the hostpath and name are required
	hostPath string

	// Path in container
	path string
	name string
}

// Validate we have an executable provided, and args and optional
func (v *HostPathVolume) Validate() bool {
	if v.hostPath == "" {
		logger.Error("The volume-hostpath addon requires a 'hostPath' for the host path.")
		return false
	}
	return v.DefaultValidate()
}

// Set custom options / attributes
func (v *HostPathVolume) SetOptions(metric *api.MetricAddon) {

	// Name is required!
	path, ok := metric.Options["hostPath"]
	if ok {
		v.hostPath = path.StrVal
	}
	path, ok = metric.Options["path"]
	if ok {
		v.path = path.StrVal
	}

	name, ok := metric.Options["name"]
	if ok {
		v.name = name.StrVal
	}
}

// AssembleVolumes for a host volume
func (v *HostPathVolume) AssembleVolumes() []specs.VolumeSpec {
	volume := corev1.Volume{
		Name: v.name,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: v.hostPath,
			},
		},
	}
	return []specs.VolumeSpec{{
		Volume:   volume,
		Mount:    true,
		Path:     v.path,
		ReadOnly: v.readOnly,
	}}
}

// An empty volume requires nothing! Nice!
type EmptyVolume struct {
	VolumeBase
	name string
	path string
}

// Validate we have an executable provided, and args and optional
func (v *EmptyVolume) Validate() bool {
	if v.name == "" {
		logger.Error("The volume-empty addon requires a 'name'.")
		return false
	}
	return v.DefaultValidate()
}

// Set custom options / attributes
func (v *EmptyVolume) SetOptions(metric *api.MetricAddon) {
	name, ok := metric.Options["name"]
	if ok {
		v.name = name.StrVal
	}
}

// AssembleVolumes for an empty volume
func (v *EmptyVolume) AssembleVolumes() []specs.VolumeSpec {
	volume := corev1.Volume{
		Name: v.name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	return []specs.VolumeSpec{{
		Volume:   volume,
		Mount:    true,
		Path:     v.path,
		ReadOnly: v.readOnly,
	}}

}

// TODO likely we need to carry around entrypoints to customize?

func init() {

	// Config map volume type
	base := AddonBase{
		Identifier: "volume-cm",
		Summary:    "config map volume type",
	}
	volBase := VolumeBase{AddonBase: base}
	vol := ConfigMapVolume{VolumeBase: volBase}
	Register(&vol)

	// Secret volume type
	base = AddonBase{
		Identifier: "volume-secret",
		Summary:    "secret volume type",
	}
	volBase = VolumeBase{AddonBase: base}
	secretVol := SecretVolume{VolumeBase: volBase}
	Register(&secretVol)

	// persistent volume claim volume type
	base = AddonBase{
		Identifier: "volume-pvc",
		Summary:    "persistent volume claim volume type",
	}
	volBase = VolumeBase{AddonBase: base}
	pvcVol := PersistentVolumeClaim{VolumeBase: volBase}
	Register(&pvcVol)

	// EmptyVolume
	base = AddonBase{
		Identifier: "volume-empty",
		Summary:    "empty volume type",
	}
	volBase = VolumeBase{AddonBase: base}
	emptyVol := EmptyVolume{VolumeBase: volBase}
	Register(&emptyVol)

}