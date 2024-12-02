/*
Copyright 2023 Lawrence Livermore National Security, LLC
 (c.f. AUTHORS, NOTICE.LLNS, COPYING)

SPDX-License-Identifier: MIT
*/

package perf

import (
	"fmt"
	"strconv"

	api "github.com/converged-computing/metrics-operator/api/v1alpha2"
	"github.com/converged-computing/metrics-operator/pkg/metadata"
	metrics "github.com/converged-computing/metrics-operator/pkg/metrics"
	"github.com/converged-computing/metrics-operator/pkg/specs"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	pidstatIdentifier = "perf-sysstat"
	pidstatSummary    = "statistics for Linux tasks (processes) : I/O, CPU, memory, etc."
	pidstatContainer  = "ghcr.io/converged-computing/metric-sysstat:latest"
)

// sysstat provides a tool "pidstat" that can monitor a PID (along with others)
// https://github.com/sysstat/sysstat

type PidStat struct {
	metrics.SingleApplication

	// Custom Options
	useColor    bool
	showPIDS    bool
	useThreads  bool
	rate        int32
	completions int32
	command     string
	commands    map[string]intstr.IntOrString
}

func (m PidStat) Url() string {
	return "https://github.com/sysstat/sysstat"
}

// Set custom options / attributes for the metric
func (m *PidStat) SetOptions(metric *api.Metric) {

	m.Identifier = pidstatIdentifier
	m.Summary = pidstatSummary
	m.Container = pidstatContainer

	// Defaults for rate and completions
	m.rate = 10
	m.completions = 0 // infinite
	m.ResourceSpec = &metric.Resources
	m.AttributeSpec = &metric.Attributes

	// Custom commands based on index of job
	m.commands = map[string]intstr.IntOrString{}

	// UseColor set to anything means to use it
	_, ok := metric.Options["color"]
	if ok {
		m.useColor = true
	}
	_, ok = metric.Options["pids"]
	if ok {
		m.showPIDS = true
	}
	_, ok = metric.Options["threads"]
	if ok {
		m.useThreads = true
	}
	rate, ok := metric.Options["rate"]
	if ok {
		m.rate = rate.IntVal
	}
	completions, ok := metric.Options["completions"]
	if ok {
		m.completions = completions.IntVal
	}

	// Parse map options
	commands, ok := metric.MapOptions["commands"]
	if ok {
		m.commands = commands
	}
	command, ok := metric.Options["command"]
	if ok {
		m.command = command.StrVal
	}

}

// Exported options and list options
func (m PidStat) Options() map[string]intstr.IntOrString {

	// Prepare bool options
	showPIDS := "false"
	if m.showPIDS {
		showPIDS = "true"
	}
	useThreads := "false"
	if m.useThreads {
		useThreads = "true"
	}

	return map[string]intstr.IntOrString{
		"rate":        intstr.FromInt(int(m.rate)),
		"completions": intstr.FromInt(int(m.completions)),
		"threads":     intstr.FromString(useThreads),
		"pids":        intstr.FromString(showPIDS),
	}
}

func (m PidStat) prepareIndexedCommand(spec *api.MetricSet) string {

	var command string
	if len(m.commands) == 0 {

		// This is a global command for the entire application
		command = fmt.Sprintf("command=\"%s\"\n", m.command)

	} else {

		// Keep a lookup of index -> command.
		// Parse "all" or other TBA global identifiers first
		commands := map[string]string{}
		for key, value := range m.commands {

			// We currently have support for all
			if key == "all" {
				for i := 0; i < int(spec.Spec.Pods); i++ {
					commands[strconv.Itoa(i)] = value.StrVal
				}
			}
		}
		// Now add commands specific to indices
		for key, value := range m.commands {
			if key == "all" {
				continue
			}
			commands[key] = value.StrVal
		}

		// Assemble final logic
		for index, cmd := range commands {
			command += fmt.Sprintf("if [[ \"JOB_COMPLETION_INDEX\" -eq %s ]]; then\n  command=\"%s\"\nfi\n", index, cmd)
		}
	}
	return command
}

func (m PidStat) PrepareContainers(
	spec *api.MetricSet,
	metric *metrics.Metric,
) []*specs.ContainerSpec {

	// Metadata to add to beginning of run
	meta := metrics.Metadata(spec, metric)

	useColor := ""
	if !m.useColor {
		useColor = "export NO_COLOR=true"
	}

	showPIDS := ""
	if m.showPIDS {
		showPIDS = "ps aux\npstree ${pid}"
	}

	useThreads := ""
	if m.useThreads {
		useThreads = " -t "
	}

	command := m.prepareIndexedCommand(spec)
	preBlock := `#!/bin/bash

echo "%s"
# Download the wait binary
wget -q https://github.com/converged-computing/goshare/releases/download/2023-07-27/wait > /dev/null
chmod +x ./wait
mv ./wait /usr/bin/goshare-wait
	
# Do we want to use threads?
threads="%s"
	
# This is logic to determine the command, it will set $command
# We do this because command to watch can vary between worker pods
%s
echo "PIDSTAT COMMAND START"
echo "$command"
echo "PIDSTAT COMMAND END"
echo "Waiting for application PID..."
pid=$(goshare-wait -c "$command" -q)
	
# Set color or not
%s
	
# See https://kellyjonbrazil.github.io/jc/docs/parsers/pidstat
# for how we get lovely json
i=0
completions=%d
echo "%s"
while true
  do
	echo "%s"
	%s
	echo "CPU STATISTICS TASK"
	pidstat -p ${pid} -u -h $threads -T TASK | jc --pidstat
	echo "CPU STATISTICS CHILD"
	pidstat -p ${pid} -u -h $threads -T CHILD | jc --pidstat
	echo "IO STATISTICS"
	pidstat -p ${pid} -d -h $threads -T ALL | jc --pidstat
	echo "POLICY"
	pidstat -p ${pid} -R -h $threads -T ALL | jc --pidstat
	echo "PAGEFAULTS TASK"
	pidstat -p ${pid} -r -h $threads -T TASK | jc --pidstat
	echo "PAGEFAULTS CHILD"
	pidstat -p ${pid} -r -h $threads -T CHILD | jc --pidstat
	echo "STACK UTILIZATION"
	pidstat -p ${pid} -s -h $threads -T ALL | jc --pidstat
	echo "THREADS TASK"
	pidstat -p ${pid} -h $threads -T TASK | jc --pidstat
	echo "THREADS CHILD"
	pidstat -p ${pid} -h $threads -T CHILD | jc --pidstat
	echo "KERNEL TABLES"
	pidstat -p ${pid} -v -h $threads -T ALL | jc --pidstat
	echo "TASK SWITCHING"
	pidstat -p ${pid} -w -h $threads -T ALL | jc --pidstat
	# Check if still running
	ps -p ${pid} > /dev/null
	retval=$?
	if [[ $retval -ne 0 ]]; then
		echo "%s"
		%s
		exit 0
	fi
	if [[ $completions -ne 0 ]] && [[ $i -eq $completions ]]; then
		echo "%s"
		%s
		exit 0
	fi
	sleep %d
	let i=i+1
done
`

	interactive := metadata.Interactive(spec.Spec.Logging.Interactive)
	preBlock = fmt.Sprintf(
		preBlock,
		meta,
		useThreads,
		command,
		useColor,
		m.completions,
		metadata.CollectionStart,
		metadata.Separator,
		showPIDS,
		metadata.CollectionEnd,
		interactive,
		metadata.CollectionEnd,
		interactive,
		m.rate,
	)
	postBlock := fmt.Sprintf("\n%s\n", interactive)
	return m.ApplicationContainerSpec(preBlock, command, postBlock)
}

func init() {
	base := metrics.BaseMetric{
		Identifier: pidstatIdentifier,
		Summary:    pidstatSummary,
		Container:  pidstatContainer,
	}
	app := metrics.SingleApplication{BaseMetric: base}
	pidstat := PidStat{SingleApplication: app}
	metrics.Register(&pidstat)
}
