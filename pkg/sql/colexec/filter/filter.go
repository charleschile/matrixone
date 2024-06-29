// Copyright 2021 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filter

import (
	"bytes"
	"fmt"

	"github.com/matrixorigin/matrixone/pkg/pb/plan"
	plan2 "github.com/matrixorigin/matrixone/pkg/sql/plan"
	"github.com/matrixorigin/matrixone/pkg/vm"

	"github.com/matrixorigin/matrixone/pkg/common/moerr"
	"github.com/matrixorigin/matrixone/pkg/container/batch"
	"github.com/matrixorigin/matrixone/pkg/container/vector"

	"github.com/matrixorigin/matrixone/pkg/sql/colexec"
	"github.com/matrixorigin/matrixone/pkg/vm/process"
)

const argName = "filter"

func (arg *Argument) String(buf *bytes.Buffer) {
	buf.WriteString(argName)
	buf.WriteString(fmt.Sprintf("filter(%s)", arg.E))
}

func (arg *Argument) Prepare(proc *process.Process) (err error) {
	arg.ctr = new(container)
	var filterExpr *plan.Expr

	if arg.exeExpr == nil {
		if arg.E != nil {
			filterExpr, err = plan2.ConstantFold(batch.EmptyForConstFoldBatch, plan2.DeepCopyExpr(arg.E), proc, true, true)
		}
	} else {
		filterExpr, err = plan2.ConstantFold(batch.EmptyForConstFoldBatch, plan2.DeepCopyExpr(arg.exeExpr), proc, true, true)
	}
	if err != nil {
		return err
	}
	arg.ctr.executors, err = colexec.NewExpressionExecutorsFromPlanExpressions(proc, colexec.SplitAndExprs([]*plan.Expr{filterExpr}))
	return err
}

func (arg *Argument) Call(proc *process.Process) (vm.CallResult, error) {
	if err, isCancel := vm.CancelCheck(proc); isCancel {
		return vm.CancelResult, err
	}

	result, err := arg.GetChildren(0).Call(proc)
	if err != nil {
		return result, err
	}

	anal := proc.GetAnalyze(arg.GetIdx(), arg.GetParallelIdx(), arg.GetParallelMajor())
	anal.Start()
	defer anal.Stop()

	if result.Batch == nil || result.Batch.IsEmpty() || result.Batch.Last() {
		return result, nil
	}
	if arg.ctr.buf != nil {
		proc.PutBatch(arg.ctr.buf)
		arg.ctr.buf = nil
	}
	arg.ctr.buf = result.Batch

	anal.Input(arg.ctr.buf, arg.GetIsFirst())

	var sels []int64
	for i := range arg.ctr.executors {
		if arg.ctr.buf.IsEmpty() {
			break
		}

		vec, err := arg.ctr.executors[i].Eval(proc, []*batch.Batch{arg.ctr.buf}, nil)
		if err != nil {
			result.Batch = nil
			return result, err
		}

		if proc.OperatorOutofMemory(int64(vec.Size())) {
			return result, moerr.NewOOM(proc.Ctx)
		}
		anal.Alloc(int64(vec.Size()))
		if !vec.GetType().IsBoolean() {
			return result, moerr.NewInvalidInput(proc.Ctx, "filter condition is not boolean")
		}

		bs := vector.GenerateFunctionFixedTypeParameter[bool](vec)
		if vec.IsConst() {
			v, null := bs.GetValue(0)
			if null || !v {
				arg.ctr.buf, err = tryDupBatch(proc, arg.ctr.buf)
				if err != nil {
					return result, err
				}
				arg.ctr.buf.Shrink(nil, false)
			}
		} else {
			if sels == nil {
				sels = proc.Mp().GetSels()
			}
			sels = sels[:0]

			l := uint64(vec.Length())
			if bs.WithAnyNullValue() {
				for j := uint64(0); j < l; j++ {
					v, null := bs.GetValue(j)
					if !null && v {
						sels = append(sels, int64(j))
					}
				}
			} else {
				for j := uint64(0); j < l; j++ {
					v, _ := bs.GetValue(j)
					if v {
						sels = append(sels, int64(j))
					}
				}
			}
			arg.ctr.buf, err = tryDupBatch(proc, arg.ctr.buf)
			if err != nil {
				return result, err
			}
			arg.ctr.buf.Shrink(sels, false)
		}
	}

	if sels != nil {
		proc.Mp().PutSels(sels)
	}

	// bad design here. should compile a pipeline like `-> restrict -> output (just do clean work or memory reuse) -> `
	// but not use the IsEnd flag to do the clean work.
	if arg.IsEnd {
		result.Batch = nil
	} else {
		anal.Output(arg.ctr.buf, arg.GetIsLast())
		if arg.ctr.buf == result.Batch {
			arg.ctr.buf = nil
		} else {
			result.Batch = arg.ctr.buf
		}
	}
	return result, nil
}

func tryDupBatch(proc *process.Process, bat *batch.Batch) (*batch.Batch, error) {
	cnt := bat.GetCnt()
	if cnt == 1 {
		return bat, nil
	}
	newBat, err := bat.Dup(proc.Mp())
	if err != nil {
		return nil, err
	}
	// proc.PutBatch(bat)
	return newBat, nil
}
