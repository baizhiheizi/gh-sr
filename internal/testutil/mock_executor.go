// Package testutil provides shared helpers for tests across gh-sr packages.
package testutil

// MockExecutor implements host.Executor for tests (see Run/Upload/Close below).
type MockExecutor struct {
	Output    string
	RunErr    error
	UploadErr error
	CloseErr  error

	RunFn     func(cmd string) (string, error)
	Responses []string
	respIdx   int

	Calls []string

	UploadCalled bool
	LastUpload   struct {
		Local, Remote string
	}
}

func (m *MockExecutor) Run(cmd string) (string, error) {
	m.Calls = append(m.Calls, cmd)
	if m.RunFn != nil {
		return m.RunFn(cmd)
	}
	if m.RunErr != nil {
		return "", m.RunErr
	}
	if len(m.Responses) > 0 {
		if m.respIdx >= len(m.Responses) {
			return "", nil
		}
		out := m.Responses[m.respIdx]
		m.respIdx++
		return out, nil
	}
	return m.Output, nil
}

func (m *MockExecutor) Upload(localPath, remotePath string) error {
	m.UploadCalled = true
	m.LastUpload.Local = localPath
	m.LastUpload.Remote = remotePath
	return m.UploadErr
}

func (m *MockExecutor) Close() error {
	return m.CloseErr
}
