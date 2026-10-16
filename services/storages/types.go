package storages

import "os"

const (
	dirPermission  = 0o750
	filePermission = 0o600
)

// Service stores files under keys, e.g. raw/touch/<sha256>.json.gz.
type Service interface {
	Exists(key string) bool
	Put(key string, body []byte) error
	Get(key string) ([]byte, error)
	Delete(key string) error
	List(prefix string) ([]string, error)
	Shutdown()
}

// Impl stores files on the volume. os.Root keeps every key inside the storage path.
type Impl struct {
	root *os.Root
}
