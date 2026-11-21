// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

//go:build docker
// +build docker

package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/security/securitytest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/redact"
)

// sqlQuery consists of a sql query and the expected result.
type sqlQuery struct {
	query          string
	expectedResult string
}

// runContainerArgs are equivalent to arguments passed to a `docker run`
// command.
type runContainerArgs struct {
	// envSetting is to set the environment variables for the docker container.
	envSetting []string
	// volSetting is to set how local directories will be mounted to the container.
	volSetting []string
	// cmd is the command to run when starting the container.
	cmd []string
}

// singleNodeDockerTest consists of two main parts: start the container with
// a single-node cockroach server using runContainerArgs,
// and execute sql queries in this running container.
type singleNodeDockerTest struct {
	testName         string
	runContainerArgs runContainerArgs
	containerName    string
	// sqlOpts are arguments passed to a `cockroach sql` command.
	sqlOpts []string
	// sqlQueries are queries to run in this container, and their expected results.
	sqlQueries []sqlQuery
}

func TestSingleNodeDocker(t *testing.T) {
	ctx := context.Background()
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatal(errors.NewAssertionErrorWithWrappedErrf(err, "cannot get pwd"))
	}

	fsnotifyBinPath := filepath.Join(pwd, "docker-fsnotify/docker-fsnotify-bin")

	// The secure container gets the seeded test certs. Its config's client
	// cert is root's, which the entrypoint uses to set up the database.
	certsDir := t.TempDir()
	for name, file := range map[string]string{
		"ca.crt": "ca.crt", "node.crt": "node.crt", "node.key": "node.key", "rules": "rules",
		"client.crt": "client.root.crt", "client.key": "client.root.key",
		"client.testuser.crt": "client.testuser.crt", "client.testuser.key": "client.testuser.key",
	} {
		b, err := securitytest.File(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(certsDir, name), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(certsDir, "config.yaml"), []byte(
		"cert: node.crt\nkey: node.key\nnode-ca: ca.crt\nclient-ca: ca.crt\nrules: rules\n"+
			"client-cert: client.crt\nclient-key: client.key\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var dockerTests = []singleNodeDockerTest{
		{
			testName:      "single-node-secure-mode",
			containerName: "roach1",
			runContainerArgs: runContainerArgs{
				envSetting: []string{
					"COCKROACH_DATABASE=mydb",
					"COCKROACH_USER=testuser",
					// The container user is not the files' owner.
					"COCKROACH_SKIP_KEY_PERMISSION_CHECK=true",
				},
				volSetting: []string{
					fmt.Sprintf("%s/testdata/single-node-test/docker-entrypoint-initdb.d/:/docker-entrypoint-initdb.d", pwd),
					fmt.Sprintf("%s:/cockroach/docker-fsnotify", fsnotifyBinPath),
					fmt.Sprintf("%s:/certs", certsDir),
				},
				cmd: []string{"start-single-node", "--config=/certs/config.yaml"},
			},
			sqlOpts: []string{
				"--format=csv",
				"--url=postgresql://testuser@127.0.0.1:26257/mydb?sslcert=%2Fcerts%2Fclient.testuser.crt&sslkey=%2Fcerts%2Fclient.testuser.key&sslmode=verify-full&sslrootcert=%2Fcerts%2Fca.crt",
			},
			sqlQueries: []sqlQuery{
				{"SELECT current_user", "current_user\ntestuser"},
				{"SELECT current_database()", "current_database\nmydb"},
				{"CREATE TABLE hello (X INT)", "CREATE TABLE"},
				{"INSERT INTO hello VALUES (1), (2), (3)", "INSERT 0 3"},
				{"SELECT * FROM hello", "x\n1\n2\n3"},
				{"SELECT * FROM bello", "id,name\n1,a\n2,b\n3,c"},
			},
		},
		{
			testName:      "single-node-insecure-mode",
			containerName: "roach2",
			runContainerArgs: runContainerArgs{
				envSetting: []string{
					"COCKROACH_DATABASE=mydb",
				},
				volSetting: []string{
					fmt.Sprintf("%s/testdata/single-node-test/docker-entrypoint-initdb.d/:/docker-entrypoint-initdb.d", pwd),
					fmt.Sprintf("%s:/cockroach/docker-fsnotify", fsnotifyBinPath),
				},
				cmd: []string{"start-single-node", "--insecure"},
			},
			sqlOpts: []string{
				"--format=csv",
				"--insecure",
				"--database=mydb",
			},
			sqlQueries: []sqlQuery{
				{"SELECT current_user", "current_user\nroot"},
				{"SELECT current_database()", "current_database\nmydb"},
				{"CREATE TABLE hello (X INT)", "CREATE TABLE"},
				{"INSERT INTO hello VALUES (1), (2), (3)", "INSERT 0 3"},
				{"SELECT * FROM hello", "x\n1\n2\n3"},
				{"SELECT * FROM bello", "id,name\n1,a\n2,b\n3,c"},
			},
		},
		{
			testName:      "single-node-insecure-mem-mode",
			containerName: "roach3",
			runContainerArgs: runContainerArgs{
				envSetting: []string{
					"COCKROACH_DATABASE=mydb",
				},
				volSetting: []string{
					fmt.Sprintf("%s/testdata/single-node-test/docker-entrypoint-initdb.d/:/docker-entrypoint-initdb.d", pwd),
					fmt.Sprintf("%s:/cockroach/docker-fsnotify", fsnotifyBinPath),
				},
				cmd: []string{"start-single-node", "--insecure", "--store=type=mem,size=0.25"},
			},
			sqlOpts: []string{
				"--format=csv",
				"--insecure",
				"--database=mydb",
			},
			sqlQueries: []sqlQuery{
				{"SELECT current_user", "current_user\nroot"},
				{"SELECT current_database()", "current_database\nmydb"},
				{"CREATE TABLE hello (X INT)", "CREATE TABLE"},
				{"INSERT INTO hello VALUES (1), (2), (3)", "INSERT 0 3"},
				{"SELECT * FROM hello", "x\n1\n2\n3"},
				{"SELECT * FROM bello", "id,name\n1,a\n2,b\n3,c"},
			},
		},
	}

	var dn dockerNode

	if err := removeLocalData(); err != nil {
		t.Fatal(err)
	}

	if err := timeutil.RunWithTimeout(
		ctx,
		"remove all containers using current image",
		defaultTimeout,
		func(ctx context.Context) error {
			return dn.removeAllContainers(ctx)
		}); err != nil {
		t.Errorf("%v", err)
	}

	for _, test := range dockerTests {
		t.Run(test.testName, func(t *testing.T) {

			if err := timeutil.RunWithTimeout(
				ctx,
				"start container",
				defaultTimeout,
				func(ctx context.Context) error {
					return dn.startContainer(
						ctx,
						test.containerName,
						test.runContainerArgs.envSetting,
						test.runContainerArgs.volSetting,
						test.runContainerArgs.cmd,
					)
				},
			); err != nil {
				t.Fatal(err)
			}

			if err := timeutil.RunWithTimeout(
				ctx,
				"wait for the server to finish the initialization",
				waitInitTimeout,
				func(ctx context.Context) error {
					return dn.waitInitFinishes(ctx)
				},
			); err != nil {
				t.Fatal(err)
			}

			if err := timeutil.RunWithTimeout(
				ctx,
				"show log",
				defaultTimeout,
				func(ctx context.Context) error {
					return dn.showContainerLog(ctx, fmt.Sprintf("%s.log", test.testName))
				},
			); err != nil {
				log.Warningf(ctx, "cannot show container log: %v", err)
			}

			for _, qe := range test.sqlQueries {
				query := qe.query
				expected := qe.expectedResult

				if err := timeutil.RunWithTimeout(
					ctx,
					redact.Sprintf("execute command \"%s\"", query),
					defaultTimeout,
					func(ctx context.Context) error {
						resp, err := dn.execSQLQuery(ctx, query, test.sqlOpts)
						if err != nil {
							return err
						}
						if err := checkQueryResult(resp.stdOut, expected); err != nil {
							return err
						}
						return nil
					},
				); err != nil {
					t.Errorf("%v", err)
				}
			}

			if err := timeutil.RunWithTimeout(
				ctx,
				"remove current container",
				defaultTimeout,
				func(ctx context.Context) error {
					return dn.rmContainer(ctx)
				},
			); err != nil {
				t.Errorf("%v", err)
			}

		})
	}

}

const (
	imageName           = "cockroachdb/cockroach-ci:latest"
	defaultTimeout      = 10 * time.Second
	waitInitTimeout     = 80 * time.Second
	initSuccessFile     = "init_success"
	cockroachEntrypoint = "./cockroach"
	hostPort            = "8080"
	cockroachPort       = "26257"
	hostIP              = "127.0.0.1"
)

// dockerNode runs one container through the docker command line.
type dockerNode struct {
	contID string
}

// docker runs the docker command line with args and returns its stdout.
func docker(ctx context.Context, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), errors.Wrapf(err, "docker %s: %s", strings.Join(args, " "), stderr.String())
	}
	return stdout.String(), nil
}

// removeLocalData removes existing database saved in cockroach-data.
func removeLocalData() error {
	err := os.RemoveAll("./cockroach-data")
	if err != nil {
		return errors.Wrap(err, "cannot remove local data")
	}
	return nil
}

// showContainerLog outputs the container's logs to the logFile and stderr.
func (dn *dockerNode) showContainerLog(ctx context.Context, logFileName string) (retErr error) {
	cmdLog, err := os.Create(logFileName)
	if err != nil {
		return errors.Wrap(err, "cannot create log file")
	}
	defer func() { retErr = errors.CombineErrors(retErr, cmdLog.Close()) }()
	cmd := exec.CommandContext(ctx, "docker", "logs", dn.contID)
	out := io.MultiWriter(cmdLog, os.Stderr)
	cmd.Stdout = out
	cmd.Stderr = out
	return errors.Wrap(cmd.Run(), "cannot show docker logs")
}

// startContainer starts a container with given setting for environment
// variables, mounted volumes, and command to run.
func (dn *dockerNode) startContainer(
	ctx context.Context, containerName string, envSetting []string, volSetting []string, cmd []string,
) error {
	args := []string{"run", "--detach", "--name", containerName, "--hostname", containerName,
		"--publish", hostIP + ":" + hostPort + ":" + hostPort,
		"--publish", hostIP + ":" + cockroachPort + ":" + cockroachPort}
	for _, env := range envSetting {
		args = append(args, "--env", env)
	}
	for _, vol := range volSetting {
		args = append(args, "--volume", vol)
	}
	args = append(args, imageName)
	args = append(args, cmd...)
	out, err := docker(ctx, args...)
	if err != nil {
		return errors.Wrap(err, "cannot start container")
	}
	dn.contID = strings.TrimSpace(out)
	return nil
}

// removeAllContainers removes all running containers based on the cockroach-ci
// docker image by force.
func (dn *dockerNode) removeAllContainers(ctx context.Context) error {
	out, err := docker(ctx, "ps", "--all", "--quiet", "--filter", "ancestor="+imageName)
	if err != nil {
		return errors.Wrapf(err, "cannot list all containers on docker image %s", imageName)
	}
	for _, id := range strings.Fields(out) {
		if _, err := docker(ctx, "rm", "--force", id); err != nil {
			return errors.Wrapf(err, "cannot remove container %s", id)
		}
	}
	return nil
}

type execResult struct {
	stdOut   string
	stdErr   string
	exitCode int
}

// execCommand is to execute command in the current container, and returns the
// execution result and possible error.
func (dn *dockerNode) execCommand(
	ctx context.Context, cmd []string, workingDir string,
) (*execResult, error) {
	args := append([]string{"exec", "--user", "root", "--workdir", workingDir, dn.contID}, cmd...)
	var stdout, stderr bytes.Buffer
	c := exec.CommandContext(ctx, "docker", args...)
	c.Stdout = &stdout
	c.Stderr = &stderr
	err := c.Run()
	res := &execResult{stdOut: stdout.String(), stdErr: stderr.String()}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.exitCode = exitErr.ExitCode()
		return res, errors.Newf("command \"%s\" exit with code %d:\n %+v",
			strings.Join(cmd, " "), res.exitCode, res)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "cannot execute command \"%s\"", strings.Join(cmd, " "))
	}
	return res, nil
}

// waitInitFinishes waits till the server finishes all init steps or timeout,
// whichever earlier. It keeps listening to the initSuccessFile till it is closed and
// written.
func (dn *dockerNode) waitInitFinishes(ctx context.Context) error {
	// Run the binary which listens to the /cockroach folder until the
	// initialization process has finished or timeout.
	res, err := dn.execCommand(ctx, []string{
		"./docker-fsnotify",
		"/cockroach",
		initSuccessFile,
		strconv.Itoa(int(waitInitTimeout.Seconds())),
	}, "/cockroach")
	if err != nil {
		return errors.Wrapf(err, "cannot run fsnotify to listen to %s:\nres:%#v\n", initSuccessFile, res)
	}

	if strings.Contains(res.stdOut, "finished") {
		return nil
	}
	return errors.Wrap(errors.Newf("%#v", res), "error waiting the initialization to finish")
}

// execSQLQuery executes the sql query and returns the server's output and
// possible error.
func (dn *dockerNode) execSQLQuery(
	ctx context.Context, sqlQuery string, sqlQueryOpts []string,
) (*execResult, error) {
	query := append([]string{cockroachEntrypoint, "sql", "-e", sqlQuery},
		sqlQueryOpts...,
	)

	res, err := dn.execCommand(ctx, query, "/cockroach")
	if err != nil {
		return nil, errors.Wrapf(err, "error executing query \"%s\"", sqlQuery)
	}

	return res, nil
}

// rmContainer performs a forced deletion of the current container.
func (dn *dockerNode) rmContainer(ctx context.Context) error {
	if _, err := docker(ctx, "rm", "--force", dn.contID); err != nil {
		return errors.Wrapf(err, "cannot remove container %s", dn.contID)
	}
	dn.contID = ""
	return nil
}

func checkQueryResult(queryRes string, expected string) error {
	formatted := strings.ReplaceAll(queryRes, "\r\n", "\n")
	if strings.Contains(formatted, expected) {
		return nil
	}
	return errors.Errorf("queryRes %q doesn't contain the expected string %q", queryRes, expected)
}
