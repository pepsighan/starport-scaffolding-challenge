package typed

import (
	"errors"
	"fmt"

	"github.com/tendermint/starport/starport/pkg/protoanalysis"
)

const gogoProtoFile = "gogoproto/gogo.proto"

// EnsureGogoProtoImported add the gogo.proto import in the proto file content in case it's not defined
func EnsureGogoProtoImported(protoFile string) string {
	err := protoanalysis.IsImported(protoFile, gogoProtoFile)
	if errors.Is(err, protoanalysis.ErrImportNotFound) {
		templateGogoProtoImport := `
import "%[1]v";`
		replacementGogoProtoImport := fmt.Sprintf(
			templateGogoProtoImport,
			gogoProtoFile,
		)
		return replacementGogoProtoImport
	}
	return ""
}
