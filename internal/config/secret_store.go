package config

import "errors"

// SecretsFile is the name of the encrypted secrets file, next to rig.json.
const SecretsFile = "secrets.dat"

// MaxSecretsFileSize bounds what the OS secret store will read.
const MaxSecretsFileSize = 64 << 10

const (
	secretsVersion  = 1
	maxSecretKeyLen = 128
	maxSecretLen    = 1024
	maxSecrets      = 256
)

var (
	// ErrNoOSSecrets means this system has no protected store: use
	// MemorySecrets instead.
	ErrNoOSSecrets = errors.New("no OS-protected secret store on this system")
	// ErrSecretsUnreadable means the secrets file exists but cannot be
	// decrypted or decoded (corrupt, tampered, or written by another Windows
	// user or machine). The store never overwrites such a file by itself.
	ErrSecretsUnreadable = errors.New("secrets file unreadable")
)

// secretsMagic prefixes the encrypted blob so a foreign file is rejected
// before it reaches the OS decryption API.
var secretsMagic = []byte("RCPSEC1\n")

// secretsEntropy is the DPAPI optional entropy: a program running as the
// same user cannot decrypt the blob with a bare CryptUnprotectData call. It
// is a namespace, not a secret.
var secretsEntropy = []byte("github.com/valentindoche/rcp-for-all-camera/secrets/v1")

type secretsDoc struct {
	Version int               `json:"version"`
	Secrets map[string]string `json:"secrets"`
}
