package typed

import (
	"fmt"

	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/pkg/multiformatname"
	"github.com/tendermint/starport/starport/pkg/placeholder"
)

func ModuleSimulationMsgModify(
	replacer placeholder.Replacer,
	path,
	content,
	moduleName string,
	typeName multiformatname.Name,
	msgs ...string,
) (string, error) {
	if len(msgs) == 0 {
		msgs = append(msgs, "")
	}
	for _, msg := range msgs {
		// simulation constants
		templateConst := `
const (
	opWeightMsg%[1]v%[2]v = "op_weight_msg_create_chain"
	// TODO: Determine the simulation weight value
	defaultWeightMsg%[1]v%[2]v int = 100
)`
		constSnippet := fmt.Sprintf(templateConst, msg, typeName.UpperCamel)
		var err error
		content, err = clipper.PasteCodeSnippetAt(path, content, clipper.GoSelectNewGlobalPosition, nil, constSnippet)
		if err != nil {
			return "", err
		}

		// simulation operations
		templateOp := `var weightMsg%[1]v%[2]v int
	simState.AppParams.GetOrGenerate(simState.Cdc, opWeightMsg%[1]v%[2]v, &weightMsg%[1]v%[2]v, nil,
		func(_ *rand.Rand) {
			weightMsg%[1]v%[2]v = defaultWeightMsg%[1]v%[2]v
		},
	)
	operations = append(operations, simulation.NewWeightedOperation(
		weightMsg%[1]v%[2]v,
		%[3]vsimulation.SimulateMsg%[1]v%[2]v(am.accountKeeper, am.bankKeeper, am.keeper),
	))`
		beforeReturnSnippet := fmt.Sprintf(templateOp, msg, typeName.UpperCamel, moduleName)
		content, err = clipper.PasteGoBeforeReturnSnippetAt(path, content, beforeReturnSnippet, clipper.SelectOptions{
			"functionName": "WeightedOperations",
		})
		if err != nil {
			return "", err
		}
	}
	return content, nil
}
