package apputil

import (
	"fmt"
	"io"
	"sort"

	conf "github.com/blacknon/lssh/internal/config"
	"github.com/urfave/cli"
)

func LoadConfigWithGenerateMode(c *cli.Context, stdout io.Writer, stderr io.Writer) (conf.Config, bool, error) {
	if handled, err := conf.HandleGenerateConfigMode(c.String("generate-lssh-conf"), stdout); handled {
		return conf.Config{}, true, err
	}

	cfg, err := conf.ReadWithFallback(c.String("file"), stderr)
	return cfg, false, err
}

func SortedServerNames(cfg conf.Config, operation string) (allNames []string, names []string, err error) {
	allNames = conf.GetNameList(cfg)
	names = append([]string(nil), allNames...)
	if operation != "" {
		names, err = cfg.FilterServersByOperation(names, operation)
		if err != nil {
			return nil, nil, err
		}
	}
	sort.Strings(names)

	return allNames, names, nil
}

func PrintServerList(w io.Writer, names []string) {
	fmt.Fprintln(w, "lssh Server List:")
	for _, name := range names {
		fmt.Fprintf(w, "  %s\n", name)
	}
}
