//go:build windows

package zapry

type pollingInstanceLock struct{}

func acquirePollingInstanceLock(_ string) (*pollingInstanceLock, error) {
	return &pollingInstanceLock{}, nil
}

func (l *pollingInstanceLock) Release() error {
	return nil
}
