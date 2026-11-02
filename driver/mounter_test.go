package driver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func writeTempFile(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dev")
	assert.NilError(t, os.WriteFile(path, data, 0600))
	return path
}

func Test_mounter_IsFormatted_requiresBlockDevice(t *testing.T) {
	m := &mounter{log: logrus.NewEntry(logrus.New())}

	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:    "missing device is an error, not unformatted",
			source:  filepath.Join(t.TempDir(), "missing"),
			wantErr: "no such file or directory",
		},
		{
			name:    "regular file is an error",
			source:  writeTempFile(t, nil),
			wantErr: "is not a block device",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatted, err := m.IsFormatted(tt.source)
			assert.ErrorContains(t, err, tt.wantErr)
			assert.Equal(t, formatted, false)
		})
	}
}

func Test_mounter_Format_requiresEmptyDevice(t *testing.T) {
	m := &mounter{log: logrus.NewEntry(logrus.New())}

	ext4 := make([]byte, emptyCheckBytes)
	ext4[1080], ext4[1081] = 0x53, 0xef
	lastByte := make([]byte, emptyCheckBytes)
	lastByte[emptyCheckBytes-1] = 1

	tests := []struct {
		name    string
		source  string
		wantErr string
	}{
		{
			name:    "ext4 superblock magic",
			source:  writeTempFile(t, ext4),
			wantErr: "non-zero data at offset 1080",
		},
		{
			name:    "non-zero byte at end of checked range",
			source:  writeTempFile(t, lastByte),
			wantErr: fmt.Sprintf("non-zero data at offset %d", emptyCheckBytes-1),
		},
		{
			name:    "non-zero data on device shorter than checked range",
			source:  writeTempFile(t, []byte{0, 0, 7}),
			wantErr: "non-zero data at offset 2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.Format(tt.source, "ext4")
			assert.ErrorContains(t, err, tt.wantErr)
			assert.Assert(t, errors.Is(err, errDeviceNotEmpty), "got %v, want errDeviceNotEmpty", err)
		})
	}
}

func Test_verifyEmpty(t *testing.T) {
	assert.NilError(t, verifyEmpty(writeTempFile(t, make([]byte, emptyCheckBytes))))

	err := verifyEmpty(filepath.Join(t.TempDir(), "missing"))
	assert.Assert(t, err != nil && !errors.Is(err, errDeviceNotEmpty), "got %v, want a read error", err)
}

func Test_mounter_WaitForDevice(t *testing.T) {
	m := &mounter{log: logrus.NewEntry(logrus.New())}

	link := filepath.Join(t.TempDir(), "scsi-0DO_Volume_vol")
	assert.NilError(t, os.Symlink(writeTempFile(t, nil), link))

	tests := []struct {
		name    string
		ctx     func() context.Context
		path    string
		wantErr string
	}{
		{
			name:    "symlink never appears",
			path:    filepath.Join(t.TempDir(), "missing"),
			wantErr: "no such file or directory",
		},
		{
			name:    "symlink to non-block device",
			path:    link,
			wantErr: "is not a block device",
		},
		{
			name: "context cancelled",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			path:    filepath.Join(t.TempDir(), "missing"),
			wantErr: "did not become ready",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.ctx != nil {
				ctx = tt.ctx()
			}

			_, err := m.WaitForDevice(ctx, tt.path, 300*time.Millisecond)
			assert.ErrorContains(t, err, "did not become ready")
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

type testAttachmentValidator struct {
	readFileFunc     func(name string) ([]byte, error)
	evalSymlinksFunc func(path string) (string, error)
}

func (av *testAttachmentValidator) readFile(name string) ([]byte, error) {
	return av.readFileFunc(name)
}

func (av *testAttachmentValidator) evalSymlinks(path string) (string, error) {
	return av.evalSymlinksFunc(path)
}

func Test_mounter_IsAttached(t *testing.T) {
	testSource := "test-source"
	testEvalSymlinkErr := errors.New("eval sym link err")
	testReadFileErr := errors.New("read file err")

	tests := []struct {
		name     string
		av       AttachmentValidator
		errorMsg string
	}{
		{
			name: "could not evaluate the symbolic link",
			av: &testAttachmentValidator{
				readFileFunc: func(name string) ([]byte, error) {
					return nil, testReadFileErr
				},
				evalSymlinksFunc: func(path string) (string, error) {
					return "", testEvalSymlinkErr
				},
			},
			errorMsg: fmt.Sprintf("error evaluating the symbolic link %q: %s", testSource, testEvalSymlinkErr),
		},
		{
			name: "error reading the device state file",
			av: &testAttachmentValidator{
				readFileFunc: func(name string) ([]byte, error) {
					return nil, testReadFileErr
				},
				evalSymlinksFunc: func(path string) (string, error) {
					return "/dev/sda", nil
				},
			},
			errorMsg: fmt.Sprintf("error reading the device state file \"/sys/class/block/sda/device/state\": %s", testReadFileErr),
		},
		{
			name: "error device name is empty",
			av: &testAttachmentValidator{
				readFileFunc: func(name string) ([]byte, error) {
					return nil, testReadFileErr
				},
				evalSymlinksFunc: func(path string) (string, error) {
					return "/", nil
				},
			},
			errorMsg: "error device name is empty for path /",
		},
		{
			name: "state file content does not indicate a running state",
			av: &testAttachmentValidator{
				readFileFunc: func(name string) ([]byte, error) {
					return []byte("not-running"), nil
				},
				evalSymlinksFunc: func(path string) (string, error) {
					return "/dev/sda", nil
				},
			},
			errorMsg: fmt.Sprintf("error comparing the state file content, expected: %s, got: %s", runningState, "not-running"),
		},
		{
			name: "state file content does not indicate a running state",
			av: &testAttachmentValidator{
				readFileFunc: func(name string) ([]byte, error) {
					return []byte("not-running"), nil
				},
				evalSymlinksFunc: func(path string) (string, error) {
					return "/dev/sda", nil
				},
			},
			errorMsg: fmt.Sprintf("error comparing the state file content, expected: %s, got: %s", runningState, "not-running"),
		},
		{
			name: "state file content with trailing newline indicates a running state",
			av: &testAttachmentValidator{
				readFileFunc: func(name string) ([]byte, error) {
					return []byte(runningState + "\n"), nil
				},
				evalSymlinksFunc: func(path string) (string, error) {
					return "/dev/sda", nil
				},
			},
			errorMsg: "",
		},
		{
			name: "state file content indicates a running state",
			av: &testAttachmentValidator{
				readFileFunc: func(name string) ([]byte, error) {
					return []byte(runningState), nil
				},
				evalSymlinksFunc: func(path string) (string, error) {
					return "/dev/sda", nil
				},
			},
			errorMsg: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mounter{
				log:                 logrus.NewEntry(logrus.New()),
				attachmentValidator: tt.av,
			}
			err := m.IsAttached(testSource)

			if tt.errorMsg != "" {
				assert.ErrorContains(t, err, tt.errorMsg)
			} else {
				assert.NilError(t, err, "should not received an error")
			}
		})
	}
}
