//go:build !windows

package telegram

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const pollingLockDirName = "zapry-agent-locks"

type pollingInstanceLock struct {
	file *os.File
	path string
}

func acquirePollingInstanceLock(botToken string) (*pollingInstanceLock, error) {
	if botToken == "" {
		return nil, errors.New("bot token is empty")
	}

	lockDir := filepath.Join(os.TempDir(), pollingLockDirName)
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}

	sum := sha256.Sum256([]byte(botToken))
	lockPath := filepath.Join(lockDir, fmt.Sprintf("polling-%x.lock", sum[:8]))
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			if ownerPID, ok := readLockOwnerPID(lockPath); ok {
				return nil, fmt.Errorf("another polling instance is already running for this bot token (owner_pid=%d, lock=%s)", ownerPID, lockPath)
			}
			return nil, fmt.Errorf("another polling instance is already running for this bot token (lock=%s)", lockPath)
		}
		return nil, fmt.Errorf("acquire lock file: %w", err)
	}

	if err := f.Truncate(0); err == nil {
		_, _ = f.Seek(0, 0)
		_, _ = fmt.Fprintf(f, "pid=%d\n", os.Getpid())
	}

	return &pollingInstanceLock{file: f, path: lockPath}, nil
}

func (l *pollingInstanceLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}

	unlockErr := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil

	if unlockErr != nil {
		return fmt.Errorf("unlock %s: %w", l.path, unlockErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", l.path, closeErr)
	}
	return nil
}

func readLockOwnerPID(lockPath string) (int, bool) {
	b, err := os.ReadFile(lockPath)
	if err != nil {
		return 0, false
	}

	raw := strings.TrimSpace(string(b))
	if raw == "" {
		return 0, false
	}

	if strings.HasPrefix(raw, "pid=") {
		pid, err := strconv.Atoi(strings.TrimPrefix(raw, "pid="))
		if err == nil && pid > 0 {
			return pid, true
		}
	}

	return 0, false
}
