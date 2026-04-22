package conf

type ProvidersConfig struct {
	Paths             []string `toml:"paths" yaml:"paths"`
	Timeout           string   `toml:"timeout" yaml:"timeout"`
	MaxParallel       int      `toml:"max_parallel" yaml:"max_parallel"`
	InventoryCacheTTL string   `toml:"inventory_cache_ttl" yaml:"inventory_cache_ttl"`
	FailOpen          bool     `toml:"fail_open" yaml:"fail_open"`
	DebugLog          string   `toml:"debug_log" yaml:"debug_log"`
}
