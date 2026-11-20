// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package main

import (
	"io"
	"strings"
	"text/template"
)

// defaultCmpProjTemplate is the common base for the template used to generate
// code for default comparison projection operators. It should be used as a
// format string with one %s argument that specifies the package that the
// generated code is placed in.
const defaultCmpProjTemplate = `
// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package {{.TargetPkg}}

import (
	"github.com/cockroachdb/cockroach/pkg/col/coldata"
	"github.com/cockroachdb/cockroach/pkg/sql/colconv"
	"github.com/cockroachdb/cockroach/pkg/sql/colexec/colexeccmp"
	"github.com/cockroachdb/cockroach/pkg/sql/colexecerror"
	"github.com/cockroachdb/cockroach/pkg/sql/colexecop"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfra/execreleasable"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

{{template "defaultCmpProjOp" .}}
`

const defaultCmpProjOpsTmpl = "pkg/sql/colexec/colexecproj/default_cmp_proj_ops_tmpl.go"

type defaultCmpProjOpOverload struct {
	TargetPkg string
	// Comparison operators are always normalized so that the constant is on the
	// right side, so we skip generating the code when the constant is on the
	// left.
	IsRightConst bool
	Kind         string
}

func getDefaultCmpProjOps(overload defaultCmpProjOpOverload) generator {
	return func(inputFileContents string, wr io.Writer) error {
		s := strings.ReplaceAll(inputFileContents, "_KIND", "{{.Kind}}")

		tmpl, err := template.New("default_cmp_proj_ops").Parse(s)
		if err != nil {
			return err
		}
		tmpl, err = tmpl.Parse(defaultCmpProjTemplate)
		if err != nil {
			return err
		}
		return tmpl.Execute(wr, overload)
	}
}

func init() {
	registerGenerator(
		getDefaultCmpProjOps(
			defaultCmpProjOpOverload{
				TargetPkg:    "colexecprojconst",
				IsRightConst: true,
				Kind:         "RConst",
			}),
		"default_cmp_proj_const_op.eg.go", /* outputFile */
		defaultCmpProjOpsTmpl,
	)
	registerGenerator(
		getDefaultCmpProjOps(
			defaultCmpProjOpOverload{
				TargetPkg:    "colexecproj",
				IsRightConst: false,
				Kind:         "",
			}),
		"default_cmp_proj_op.eg.go", /* outputFile */
		defaultCmpProjOpsTmpl,
	)
}
