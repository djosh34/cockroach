// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package scmutationexec

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/funcdesc"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scop"
)

func (i *immediateVisitor) CreateFunctionDescriptor(
	ctx context.Context, op scop.CreateFunctionDescriptor,
) error {
	params := make([]descpb.FunctionDescriptor_Parameter, len(op.Function.Params))
	for i, param := range op.Function.Params {

		params[i] = descpb.FunctionDescriptor_Parameter{
			Class: param.Class.Class,
			Name:  param.Name,
			Type:  param.Type.Type,
		}
	}

	mut := funcdesc.NewMutableFunctionDescriptor(
		op.Function.FunctionID,
		descpb.InvalidID,
		descpb.InvalidID,
		"",
		params,
		op.Function.ReturnType.Type,
		op.Function.ReturnSet,
		op.Function.IsProcedure,
		&catpb.PrivilegeDescriptor{Version: catpb.Version23_2},
	)
	mut.State = descpb.DescriptorState_ADD
	i.CreateDescriptor(&mut)
	return nil
}

func (i *immediateVisitor) SetFunctionName(ctx context.Context, op scop.SetFunctionName) error {
	fn, err := i.checkOutFunction(ctx, op.FunctionID)
	if err != nil {
		return err
	}
	fn.SetName(op.Name)
	return nil
}

func (i *immediateVisitor) SetFunctionVolatility(
	ctx context.Context, op scop.SetFunctionVolatility,
) error {
	fn, err := i.checkOutFunction(ctx, op.FunctionID)
	if err != nil {
		return err
	}
	fn.SetVolatility(op.Volatility)
	return nil
}

func (i *immediateVisitor) SetFunctionLeakProof(
	ctx context.Context, op scop.SetFunctionLeakProof,
) error {
	fn, err := i.checkOutFunction(ctx, op.FunctionID)
	if err != nil {
		return err
	}
	fn.SetLeakProof(op.LeakProof)
	return nil
}

func (i *immediateVisitor) SetFunctionNullInputBehavior(
	ctx context.Context, op scop.SetFunctionNullInputBehavior,
) error {
	fn, err := i.checkOutFunction(ctx, op.FunctionID)
	if err != nil {
		return err
	}
	fn.SetNullInputBehavior(op.NullInputBehavior)
	return nil
}

func (i *immediateVisitor) SetFunctionBody(ctx context.Context, op scop.SetFunctionBody) error {
	fn, err := i.checkOutFunction(ctx, op.Body.FunctionID)
	if err != nil {
		return err
	}
	fn.SetFuncBody(op.Body.Body)
	fn.SetLang(op.Body.Lang.Lang)

	return nil
}

func (i *immediateVisitor) SetFunctionParamDefaultExpr(
	ctx context.Context, op scop.SetFunctionParamDefaultExpr,
) error {
	// TODO(chengxiong): when default parameter value is supported, we need to
	// address references here because functions, types and sequences can be used
	// with a default value expression.
	return nil
}
