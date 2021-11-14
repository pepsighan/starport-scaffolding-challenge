package query

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/genny"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/pkg/placeholder"
	"github.com/tendermint/starport/starport/pkg/xgenny"
)

// NewStargate returns the generator to scaffold a empty query in a Stargate module
func NewStargate(replacer placeholder.Replacer, opts *Options) (*genny.Generator, error) {
	var (
		g        = genny.New()
		template = xgenny.NewEmbedWalker(
			fsStargate,
			"stargate/",
			opts.AppPath,
		)
	)

	g.RunFn(protoQueryModify(opts))
	g.RunFn(cliQueryModify(replacer, opts))

	return g, Box(template, opts, g)
}

func protoQueryModify(opts *Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "proto", opts.ModuleName, "query.proto")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		// RPC service
		templateRPC := `

  // Queries a list of %[2]v items.
	rpc %[1]v(Query%[1]vRequest) returns (Query%[1]vResponse) {
		option (google.api.http).get = "/%[3]v/%[4]v/%[5]v/%[2]v";
	}
`
		replacementRPC := fmt.Sprintf(
			templateRPC,
			opts.QueryName.UpperCamel,
			opts.QueryName.LowerCamel,
			opts.OwnerName,
			opts.AppName,
			opts.ModuleName,
		)
		content, err := clipper.PasteProtoSnippetAt(
			f.String(),
			clipper.ProtoSelectNewServiceMethodPosition,
			clipper.SelectOptions{
				"name": "Query",
			},
			replacementRPC,
		)
		if err != nil {
			return err
		}

		// Fields for request
		var reqFields string
		for i, field := range opts.ReqFields {
			reqFields += fmt.Sprintf("  %s;\n", field.ProtoType(i+1))
		}
		if opts.Paginated {
			reqFields += fmt.Sprintf("cosmos.base.query.v1beta1.PageRequest pagination = %d;\n", len(opts.ReqFields)+1)
		}

		// Fields for response
		var resFields string
		for i, field := range opts.ResFields {
			resFields += fmt.Sprintf("  %s;\n", field.ProtoType(i+1))
		}
		if opts.Paginated {
			resFields += fmt.Sprintf("cosmos.base.query.v1beta1.PageResponse pagination = %d;\n", len(opts.ResFields)+1)
		}

		// Ensure custom types are imported
		protoImports := append(opts.ResFields.ProtoImports(), opts.ReqFields.ProtoImports()...)
		customFields := append(opts.ResFields.Custom(), opts.ReqFields.Custom()...)
		for _, f := range customFields {
			protoImports = append(protoImports,
				fmt.Sprintf("%[1]v/%[2]v.proto", opts.ModuleName, f),
			)
		}
		for _, f := range protoImports {
			importModule := fmt.Sprintf(`
import "%[1]v";`, f)
			content = strings.ReplaceAll(content, importModule, "")

			content, err = clipper.PasteProtoSnippetAt(
				content,
				clipper.ProtoSelectNewImportPosition,
				nil,
				importModule,
			)
			if err != nil {
				return err
			}
		}

		// Messages
		templateMessages := `

message Query%[1]vRequest {
%[2]v}

message Query%[1]vResponse {
%[3]v}`
		replacementMessages := fmt.Sprintf(
			templateMessages,
			opts.QueryName.UpperCamel,
			reqFields,
			resFields,
		)
		content, err = clipper.PasteProtoSnippetAt(content, clipper.ProtoSelectLastPosition, nil, replacementMessages)
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}

func cliQueryModify(replacer placeholder.Replacer, opts *Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "client/cli/query.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		template := `cmd.AddCommand(Cmd%[2]v())

%[1]v`
		replacement := fmt.Sprintf(
			template,
			Placeholder,
			opts.QueryName.UpperCamel,
		)
		content := replacer.Replace(f.String(), Placeholder, replacement)

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
