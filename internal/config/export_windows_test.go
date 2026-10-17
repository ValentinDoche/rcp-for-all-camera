package config

// The DPAPI helpers, for the external test that checks the entropy is
// required.
var (
	DPAPIProtect   = dpapiProtect
	DPAPIUnprotect = dpapiUnprotect
	SecretsEntropy = secretsEntropy
)
