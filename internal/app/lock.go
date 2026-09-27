package app

import (
	"errors"
	"fmt"
	"os"
)

// ErrEngineInUse means another Insight process already owns the database.
// One coordinator owns one SQLite database and data directory (ADR 0001):
// a second process would claim the first one's queued analyses and ingests
// without the settings they were enqueued with.
var ErrEngineInUse = errors.New("another Insight process is using this database")

// engineLock is an exclusive advisory lock on "<db>.lock", held for the
// life of the engine. The operating system releases it when the process
// exits, so a crash never leaves a stale lock behind.
type engineLock struct{ f *os.File }

func lockEngine(dbPath string) (*engineLock, error) {
	path := dbPath + ".lock"
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open engine lock: %w", err)
	}
	if err := tryLock(f); err != nil {
		f.Close()
		if errors.Is(err, errLockHeld) {
			return nil, fmt.Errorf("%w (%s); stop the other server or headless command, or use a different -db", ErrEngineInUse, dbPath)
		}
		return nil, fmt.Errorf("lock engine: %w", err)
	}
	return &engineLock{f: f}, nil
}

func (l *engineLock) release() error {
	if l == nil {
		return nil
	}
	unlock(l.f)
	return l.f.Close()
}
