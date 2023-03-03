package driver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

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
		wantErr  bool
		want     bool
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
			wantErr:  true,
			want:     false,
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
			wantErr:  true,
			want:     false,
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
			wantErr:  true,
			want:     false,
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
			wantErr:  true,
			want:     false,
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
			wantErr:  true,
			want:     false,
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
			wantErr:  false,
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mounter{
				log:                 logrus.NewEntry(logrus.New()),
				attachmentValidator: tt.av,
			}
			got, err := m.IsAttached(testSource)

			if tt.wantErr {
				assert.ErrorContains(t, err, tt.errorMsg)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
