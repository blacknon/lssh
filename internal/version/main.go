package version

import "fmt"

const (
	SuiteName = "lssh-suite"
	Number    = "0.7.1"
)

type Maturity string

const (
	Alpha  Maturity = "alpha"
	Beta   Maturity = "beta"
	Stable Maturity = "stable"
)

type Domain string

const (
	Unknown  Domain = "unknown"
	Core     Domain = "core"
	Transfer Domain = "transfer"
	Monitor  Domain = "monitor"
	Sysadmin Domain = "sysadmin"
)

type Info struct {
	Suite    string
	Number   string
	Maturity Maturity
	Domain   Domain
}

func (i Info) String() string {
	return fmt.Sprintf("%s %s (%s/%s)", i.Suite, i.Number, i.Maturity, i.Domain)
}

func ForCommand(name string) Info {
	info := Info{
		Suite:    SuiteName,
		Number:   Number,
		Maturity: Alpha,
		Domain:   Unknown,
	}

	switch name {
	// Core
	case "lssh":
		info.Domain = Core
		info.Maturity = Stable

	// Transfer
	case "lscp":
		info.Domain = Transfer
		info.Maturity = Stable
	case "lsftp":
		info.Domain = Transfer
		info.Maturity = Stable
	case "lssync":
		info.Domain = Transfer
		info.Maturity = Alpha

	// Monitor
	case "lsmon":
		info.Domain = Monitor
		info.Maturity = Beta

	// Sysadmin
	case "lsshell":
		info.Domain = Sysadmin
		info.Maturity = Beta
	case "lsmux":
		info.Domain = Sysadmin
		info.Maturity = Beta
	case "lsshfs":
		info.Domain = Sysadmin
		info.Maturity = Alpha
	}

	return info
}

func AppVersion(name string) string {
	return ForCommand(name).String()
}
