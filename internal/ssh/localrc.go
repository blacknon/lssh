package ssh

import conf "github.com/blacknon/lssh/internal/config"

func LocalRCEnabled(r *Run, config conf.ServerConfig) bool {
	if r == nil {
		return config.LocalRcUse == "yes"
	}
	if r.IsNotBashrc {
		return false
	}
	if r.IsBashrc {
		return true
	}
	return config.LocalRcUse == "yes"
}

func LocalRCCommand(r *Run, config conf.ServerConfig) string {
	if !LocalRCEnabled(r, config) {
		return ""
	}
	return BuildInteractiveLocalRCShellCommand(
		config.LocalRcPath,
		config.LocalRcDecodeCmd,
		config.LocalRcCompress,
		config.LocalRcUncompressCmd,
	)
}
