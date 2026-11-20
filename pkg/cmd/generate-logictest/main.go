// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/sql/logictest/logictestbase"
)

type testFileTemplateConfig struct {
	LogicTest             bool
	ExecBuildLogicTest    bool
	ForceProductionValues bool
	Package, RelDir       string
	ConfigIdx             int
}

var outDir = flag.String("out-dir", "", "path to the root of the cockroach workspace")

type logicTestGenerator struct {
	execBuildLogicTestsGlob, logicTestsGlob string
}

func (g *logicTestGenerator) testdir(rel string) (*testdir, error) {
	abs := filepath.Join(*outDir, rel)
	err := os.RemoveAll(abs)
	if err != nil {
		return nil, err
	}
	err = os.MkdirAll(abs, 0777)
	if err != nil {
		return nil, err
	}
	relPathToParent := ".."
	numSlashes := strings.Count(rel, "/")
	for numSlashes > 0 {
		relPathToParent += "/.."
		numSlashes--
	}
	return &testdir{g: g, dir: abs, relPathToParent: relPathToParent}, nil
}

type testPrefixConfigPaths struct {
	testPrefix  string
	configPaths [][]string
}

type testdir struct {
	g                                                     *logicTestGenerator
	dir, relPathToParent                                  string
	execBuildLogicTestsConfigPaths, logicTestsConfigPaths []testPrefixConfigPaths
}

func (t *testdir) addLogicTests(testPrefix string, calc logictestbase.ConfigCalculator) error {
	paths, err := calc.Enumerate(t.g.logicTestsGlob)
	if err != nil {
		return err
	}
	t.logicTestsConfigPaths = append(t.logicTestsConfigPaths, testPrefixConfigPaths{testPrefix: testPrefix, configPaths: paths})
	return nil
}

func (t *testdir) addExecBuildLogicTests(
	testPrefix string, calc logictestbase.ConfigCalculator,
) error {
	paths, err := calc.Enumerate(t.g.execBuildLogicTestsGlob)
	if err != nil {
		return err
	}
	t.execBuildLogicTestsConfigPaths = append(t.execBuildLogicTestsConfigPaths, testPrefixConfigPaths{testPrefix: testPrefix, configPaths: paths})
	return nil
}

var tmpRegexp = regexp.MustCompile(`/_(\w|\-)+$`)

func (t *testdir) dump() error {
	for configIdx := range logictestbase.LogicTestConfigs {
		tplCfg := testFileTemplateConfig{
			ForceProductionValues: strings.HasSuffix(t.dir, "pkg/sql/opt/exec/execbuilder/tests"),
		}
		var testCount int
		nonTmpCount := func(paths []string) int {
			n := 0
			for i := range paths {
				if !tmpRegexp.MatchString(paths[i]) {
					n++
				}
			}
			return n
		}
		for _, configPaths := range t.logicTestsConfigPaths {
			paths := configPaths.configPaths[configIdx]
			testCount += nonTmpCount(paths)
			if len(paths) > 0 {
				tplCfg.LogicTest = true
			}
		}
		for _, configPaths := range t.execBuildLogicTestsConfigPaths {
			paths := configPaths.configPaths[configIdx]
			testCount += nonTmpCount(paths)
			if len(paths) > 0 {
				tplCfg.ExecBuildLogicTest = true
			}
		}
		if testCount == 0 {
			continue
		}
		cfg := logictestbase.LogicTestConfigs[configIdx]
		tplCfg.ConfigIdx = configIdx
		tplCfg.Package = strings.ReplaceAll(strings.ReplaceAll(cfg.Name, "-", "_"), ".", "")
		tplCfg.RelDir = t.relPathToParent
		subdir := filepath.Join(t.dir, cfg.Name)
		if err := os.Mkdir(subdir, 0777); err != nil {
			return err
		}
		var f bytes.Buffer
		err := testFileTpl.Execute(&f, tplCfg)
		if err != nil {
			return err
		}
		for _, configPaths := range t.logicTestsConfigPaths {
			paths := configPaths.configPaths[configIdx]
			for _, file := range paths {
				dumpTestForFile(&f, configPaths.testPrefix, filepath.Base(file), "runLogicTest")
			}
		}
		for _, configPaths := range t.execBuildLogicTestsConfigPaths {
			paths := configPaths.configPaths[configIdx]
			for _, file := range paths {
				dumpTestForFile(&f, configPaths.testPrefix, filepath.Base(file), "runExecBuildLogicTest")
			}
		}
		if err := os.WriteFile(filepath.Join(subdir, "generated_test.go"), f.Bytes(), 0666); err != nil {
			return err
		}
	}
	return nil
}

func dumpTestForFile(f io.Writer, prefix, file string, whichFunc string) {
	if strings.HasPrefix(file, "_") {
		// Ignore test files that start with "_", which are reserved as
		// temporary files used during development and debugging. Tests in these
		// files can be tested with the auto-generated TestLogic_tmp test.
		return
	}
	mungedFile := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(file, "/", ""), "-", "_"), ".", "_")
	fmt.Fprintf(f, "\nfunc %s_%s(\n", prefix, mungedFile)
	fmt.Fprintln(f, "\tt *testing.T,\n) {")
	fmt.Fprintln(f, "\tdefer leaktest.AfterTest(t)()")
	fmt.Fprintf(f, "\t%s(t, \"%s\")\n", whichFunc, file)
	fmt.Fprintln(f, "}")
}

func generate() error {
	if *outDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		*outDir = cwd
	}
	gen := logicTestGenerator{
		execBuildLogicTestsGlob: "pkg/sql/opt/exec/execbuilder/testdata/[^.]*",
		logicTestsGlob:          "pkg/sql/logictest/testdata/logic_test/[^.]*",
	}

	{
		t, err := gen.testdir("pkg/sql/logictest/tests")
		if err != nil {
			return err
		}
		err = t.addLogicTests("TestLogic", logictestbase.ConfigCalculator{})
		if err != nil {
			return err
		}
		err = t.dump()
		if err != nil {
			return err
		}
	}

	{
		t, err := gen.testdir("pkg/sql/opt/exec/execbuilder/tests")
		if err != nil {
			return err
		}
		err = t.addExecBuildLogicTests("TestExecBuild", logictestbase.ConfigCalculator{})
		if err != nil {
			return err
		}
		err = t.dump()
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	flag.Parse()
	if err := generate(); err != nil {
		panic(err)
	}
}
