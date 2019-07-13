// Copyright © 2019 Alvaro Saurin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssh

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/go-cmd/cmd"
	"github.com/hashicorp/terraform/communicator/remote"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/go-linereader"
)

const (
	// arguments for "sudo"
	sudoArgs = "--non-interactive"
)

// DoExec is a runner for remote Commands
func DoExec(command string) Action {
	return ActionFunc(func(cfg Config) (res Action) {
		if len(command) == 0 {
			return nil
		}

		if cfg.UseSudo {
			command = "sudo " + sudoArgs + " " + command
		}
		command += " 2>&1"

		Debug("running %q", command)

		outR, outW := io.Pipe()
		errR, errW := io.Pipe()
		outDoneCh := make(chan struct{})
		errDoneCh := make(chan struct{})

		copyOutput := func(output terraform.UIOutput, input io.Reader, done chan<- struct{}) {
			defer close(done)
			lr := linereader.New(input)
			for line := range lr.Ch {
				output.Output(line)
			}
		}

		go copyOutput(cfg.GetExecOutput(), outR, outDoneCh)
		go copyOutput(cfg.GetExecOutput(), errR, errDoneCh)

		cmd := &remote.Cmd{
			Command: command,
			Stdout:  outW,
			Stderr:  errW,
		}

		if err := cfg.Comm.Start(cmd); err != nil {
			return ActionError(fmt.Sprintf("Error executing command %q: %v", cmd.Command, err))
		}
		waitResult := cmd.Wait()
		if waitResult != nil {
			cmdError, _ := waitResult.(*remote.ExitError)
			if cmdError.ExitStatus != 0 {
				res = ActionError(fmt.Sprintf("Command %q exited with non-zero exit status: %d", cmdError.Command, cmdError.ExitStatus))
			}
			// otherwise, it is a communicator error
		}

		// wait until the copyOutput function is done
		outW.Close()
		errW.Close()
		<-outDoneCh
		<-errDoneCh

		return
	})
}

// DoExecScript is a runner for a script (with some random path in /tmp)
func DoExecScript(contents io.Reader) Action {
	path, err := GetTempFilename()
	if err != nil {
		return ActionError(fmt.Sprintf("Could not create temporary file: %s", err))
	}
	return DoWithCleanup(
		ActionList{
			DoTry(DoDeleteFile(path)),
		},
		ActionList{
			doRealUploadFile(contents, path),
			DoExec(fmt.Sprintf("sh %s", path)),
		})
}

// DoLocalExec executes a local command
func DoLocalExec(command string, args ...string) Action {
	return ActionFunc(func(cfg Config) Action {
		output := cfg.GetExecOutput()

		fullCmd := fmt.Sprintf("%s %s", command, strings.Join(args, " "))
		cfg.UserOutput.Output(fmt.Sprintf("Running local command %q...", fullCmd))

		// Disable output buffering, enable streaming
		cmdOptions := cmd.Options{
			Buffered:  false,
			Streaming: true,
		}

		envCmd := cmd.NewCmdOptions(cmdOptions, command, args...)

		go func() {
			for {
				select {
				case line := <-envCmd.Stdout:
					output.Output(line)
				case line := <-envCmd.Stderr:
					output.Output("ERROR: " + line)
				}
			}
		}()

		// Run and wait for Cmd to return
		status := <-envCmd.Start()

		// Cmd has finished but wait for goroutine to print all lines
		for len(envCmd.Stdout) > 0 || len(envCmd.Stderr) > 0 {
			time.Sleep(10 * time.Millisecond)
		}

		if status.Exit != 0 {
			cfg.UserOutput.Output(fmt.Sprintf("Error waiting for %q: %s [%d]",
				command, status.Error.Error(), status.Exit))
			return ActionError(status.Error.Error())
		}

		return nil
	})
}

// CheckExec checks if bash command succeedes or not
func CheckExec(cmd string) CheckerFunc {
	const success = "CONDITION_SUCCEEDED"
	const failure = "CONDITION_FAILED"
	command := fmt.Sprintf("%s && echo '%s' || echo '%s'", cmd, success, failure)

	return CheckerFunc(func(cfg Config) (bool, error) {
		Debug("Checking condition: '%s'", cmd)
		var buf bytes.Buffer
		if res := DoSendingExecOutputToWriter(&buf, DoExec(command)).Apply(cfg); IsError(res) {
			Debug("ERROR: when performing check %q: %s", cmd, res)
			return false, res
		}

		// check _only_ the `success` appears, as some other error/log message about
		// the command can contain both...
		s := buf.String()
		if strings.Contains(s, success) && !strings.Contains(s, failure) {
			Debug("check %q succeeded (%q found in output)", cmd, success)
			return true, nil
		}
		Debug("check %q failed", cmd)
		return false, nil
	})
}

// CheckBinaryExists checks that a binary exists in the path
func CheckBinaryExists(cmd string) CheckerFunc {
	command := fmt.Sprintf("command -v '%s'", cmd)

	return CheckerFunc(func(cfg Config) (bool, error) {
		Debug("Checking binary exists with: '%s'", cmd)
		var buf bytes.Buffer
		if res := DoSendingExecOutputToWriter(&buf, DoExec(command)).Apply(cfg); IsError(res) {
			Debug("ERROR: when performing check: %s", res)
			return false, res
		}

		// if "command -v" doesn't print anything, it was not found
		s := strings.TrimSpace(buf.String())
		if s == "" {
			Debug("%q NOT found: empty output: output == %q", cmd, s)
			return false, nil
		}

		// sometimes it just returns the file name provided
		if s == cmd {
			Debug("%q found: output == %q", cmd, s)
			return true, nil
		}

		// if it prints the full path: check it is really there
		if path.IsAbs(s) {
			Debug("checking file %q exists at %q", cmd, s)
			return CheckFileExists(s).Check(cfg)
		}

		// otherwise, just fail
		Debug("%q NOT found: output == %q", cmd, s)
		return false, nil
	})
}
