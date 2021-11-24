package singleton

import (
	"path/filepath"

	"github.com/gobuffalo/genny"
	"github.com/tendermint/starport/starport/pkg/clipper"
	"github.com/tendermint/starport/starport/templates/typed"
)

func moduleSimulationModify(clip *clipper.Clipper, opts *typed.Options) genny.RunFn {
	return func(r *genny.Runner) error {
		path := filepath.Join(opts.AppPath, "x", opts.ModuleName, "module_simulation.go")
		f, err := r.Disk.Find(path)
		if err != nil {
			return err
		}

		content, err := typed.ModuleSimulationMsgModify(
			clip,
			path,
			f.String(),
			opts.ModuleName,
			opts.TypeName,
			"Create", "Update", "Delete",
		)
		if err != nil {
			return err
		}

		newFile := genny.NewFileS(path, content)
		return r.File(newFile)
	}
}
