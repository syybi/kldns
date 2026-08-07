package runtime

import (
	"sync"

	"kldns/internal/config"
	"kldns/internal/repository"
)

var runtime struct {
	sync.RWMutex
	db  *repository.Database
	cfg config.Config
}

func SetDB(db *repository.Database) {
	runtime.Lock()
	defer runtime.Unlock()
	runtime.db = db
}

func DB() *repository.Database {
	runtime.RLock()
	defer runtime.RUnlock()
	return runtime.db
}

func SetConfig(cfg config.Config) {
	runtime.Lock()
	defer runtime.Unlock()
	runtime.cfg = cfg
}

func Current() config.Config {
	runtime.RLock()
	defer runtime.RUnlock()
	return runtime.cfg
}

func SecretKey() string {
	cfg := Current()
	if cfg.Security.SecretKey == "" {
		return config.DefaultSecretKey
	}
	return cfg.Security.SecretKey
}
