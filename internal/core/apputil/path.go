package apputil

import (
	"fmt"
	"os"

	"github.com/blacknon/lssh/internal/check"
	"github.com/blacknon/lssh/internal/common"
)

type TransferPathSpec struct {
	IsRemote bool
	Path     string
}

type PreparedTransferPath struct {
	IsRemote    bool
	Path        string
	DisplayPath string
}

func ParseTransferPathSpec(arg string) (TransferPathSpec, error) {
	isRemote, path, err := check.ParseScpPathE(arg)
	if err != nil {
		return TransferPathSpec{}, err
	}

	return TransferPathSpec{
		IsRemote: isRemote,
		Path:     path,
	}, nil
}

func PrepareTransferSourcePaths(specs []TransferPathSpec) ([]PreparedTransferPath, error) {
	return prepareTransferPaths(specs, true)
}

func PrepareTransferDestinationPath(spec TransferPathSpec) (PreparedTransferPath, error) {
	paths, err := prepareTransferPaths([]TransferPathSpec{spec}, false)
	if err != nil {
		return PreparedTransferPath{}, err
	}

	return paths[0], nil
}

func prepareTransferPaths(specs []TransferPathSpec, requireLocalExists bool) ([]PreparedTransferPath, error) {
	paths := make([]PreparedTransferPath, 0, len(specs))
	for _, spec := range specs {
		path, err := prepareTransferPath(spec, requireLocalExists)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}

	return paths, nil
}

func prepareTransferPath(spec TransferPathSpec, requireLocalExists bool) (PreparedTransferPath, error) {
	prepared := PreparedTransferPath{
		IsRemote:    spec.IsRemote,
		Path:        spec.Path,
		DisplayPath: spec.Path,
	}

	if spec.IsRemote {
		prepared.Path = check.EscapePath(spec.Path)
		return prepared, nil
	}

	fullPath := common.GetFullPath(spec.Path)
	if requireLocalExists {
		if _, err := os.Stat(fullPath); err != nil {
			return PreparedTransferPath{}, fmt.Errorf("not found path %s", spec.Path)
		}
	}

	prepared.Path = fullPath
	return prepared, nil
}
