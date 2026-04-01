package version

import "fmt"

const (
	SuiteName = "lssh-suite"
	Number    = "0.7.0"
	State     = "stable"
)

type Info struct {
	Suite  string
	Number string
	State  string
	Domain string
}

func (i Info) String() string {
	return fmt.Sprintf("%s %s (%s/%s)", i.Suite, i.Number, i.State, i.Domain)
}

func ForCommand(name string) Info {
	info := Info{
		Suite:  SuiteName,
		Number: Number,
		State:  State,
		Domain: "ops",
	}

	switch name {
	case "lscp", "lsftp":
		info.Domain = "transfer"
	case "lsmon":
		info.Domain = "monitor"
	case "lsshell", "lsmux", "lssh":
		info.Domain = "ops"
	}

	return info
}

func AppVersion(name string) string {
	return ForCommand(name).String()
}
