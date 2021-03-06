/*
Copyright 2022 Adolfo García Veytia

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package exec

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/puerco/tejolote/pkg/watcher"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/release-utils/command"
)

type RunnerImplementation interface {
	CreateRun(*Options, Step) (*Run, error)
	Snapshot(*Options, *[]watcher.Watcher) error
	Execute(*Options, *Run) error
	WriteAttestation(*Options, *Run) error
}

type defaultRunnerImplementation struct{}

// CreateRun creates a run from the data defined in the step
func (ri *defaultRunnerImplementation) CreateRun(opts *Options, step Step) (run *Run, err error) {
	var cmd *command.Command
	cwd := opts.CWD
	if opts.CWD == "" {
		cwd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting current directory: %w", err)
		}
	}
	cmd = command.NewWithWorkDir(
		cwd,
		step.Command(),
		step.Params()...,
	)

	run = &Run{
		Executable: cmd,
		ExitCode:   0,
		Artifacts:  []watcher.Artifact{},
		Output:     &command.Stream{},
		Status:     command.Status{},
		Command:    step.Command(),
		Params:     step.Params(),
		Environment: RunEnvironment{
			Directory: cwd,
			Variables: map[string]string{},
		},
	} // command.Command

	opts.Logger.Infof(
		"Executing command: %s %s", step.Command(), strings.Join(step.Params(), " "),
	)
	return run, nil
}

func (ri *defaultRunnerImplementation) Execute(opts *Options, run *Run) (err error) {
	var output *command.Stream

	run.StartTime = time.Now()
	// Execute the run's command
	if opts.Verbose {
		output, err = run.Executable.RunSuccessOutput()
	} else {
		output, err = run.Executable.RunSilentSuccessOutput()
	}
	run.EndTime = time.Now()
	if err != nil {
		return fmt.Errorf("executing run: %w", err)
	}

	run.Output = output
	if opts.Verbose {
		logrus.Info(run.Output)
	}
	return nil
}

func (ri *defaultRunnerImplementation) Snapshot(opts *Options, watchers *[]watcher.Watcher) error {
	// Take the initial snapshots
	for i := range *watchers {
		if err := (*watchers)[i].Snap(); err != nil {
			return fmt.Errorf("snapshotting watcher: %w", err)
		}
	}
	return nil
}

func (ri *defaultRunnerImplementation) WriteAttestation(opts *Options, run *Run) error {
	path := opts.AttestationPath
	if path == "" {
		f, err := os.CreateTemp("", "provenance-*.json")
		if err != nil {
			return fmt.Errorf("creating temp file to write attestation: %w", err)
		}
		path = f.Name()
		opts.Logger.Debugf("Writing attestation to temp file: %s", path)
	}
	if err := run.WriteAttestation(path); err != nil {
		return fmt.Errorf("writing attestation path: %w", err)
	}
	opts.Logger.Infof("Wrote provenance attestation to %s", path)
	return nil
}
