package config

// OpenAICodexTicketConfig 控制 ChatGPT OAuth 的 x-codex-turn-state 门票。
// Harvesting uses the configured proxy pool; business requests can reuse the ticket exit.
// Defaults retain the existing 292-byte policy, a 240-second TTL and a 60-second refresh window.
// Explicit target_length=780 enables the new 780-byte harvesting strategy.
type OpenAICodexTicketConfig struct {
	Enabled                      bool     `mapstructure:"enabled"`
	TargetLength                 int      `mapstructure:"target_length"`
	TTLSeconds                   int      `mapstructure:"ttl_seconds"`
	RefreshBeforeSeconds         int      `mapstructure:"refresh_before_seconds"`
	HarvestProxyURL              string   `mapstructure:"harvest_proxy_url"`
	HarvestProbeIntervalSeconds  int      `mapstructure:"harvest_probe_interval_seconds"`
	HarvestCooldownSeconds       int      `mapstructure:"harvest_cooldown_seconds"`
	MaxProbesPerRound            int      `mapstructure:"max_probes_per_round"`
	HarvestAttemptTimeoutSeconds int      `mapstructure:"harvest_attempt_timeout_seconds"`
	HarvestMaxConcurrent         int      `mapstructure:"harvest_max_concurrent"`
	HarvestAccountConcurrency    int      `mapstructure:"harvest_account_concurrency"`
	HarvestProxyConcurrency      int      `mapstructure:"harvest_proxy_concurrency"`
	FailClosed                   bool     `mapstructure:"fail_closed"`
	Models                       []string `mapstructure:"models"`
}
