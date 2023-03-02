package driver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
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
	t.Run("could not evaluate the symbolic link", func(t *testing.T) {
		m := &mounter{
			log:      logrus.NewEntry(logrus.New()),
			kMounter: nil,
			attachmentValidator: &testAttachmentValidator{
				readFileFunc: nil,
				evalSymlinksFunc: func(path string) (string, error) {
					return "", errors.New("error")
				},
			},
		}

		_, err := m.IsAttached("")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("error reading the device state file", func(t *testing.T) {
		m := &mounter{
			log:      logrus.NewEntry(logrus.New()),
			kMounter: nil,
			attachmentValidator: &testAttachmentValidator{
				readFileFunc: func(name string) ([]byte, error) {
					return nil, errors.New("error")
				},
				evalSymlinksFunc: func(path string) (string, error) {
					return "/dev/sda", nil
				},
			},
		}

		_, err := m.IsAttached("")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("state file content does not indicate a running state", func(t *testing.T) {
		m := &mounter{
			log:      logrus.NewEntry(logrus.New()),
			kMounter: nil,
			attachmentValidator: &testAttachmentValidator{
				readFileFunc: func(name string) ([]byte, error) {
					return []byte("not-running\n"), nil
				},
				evalSymlinksFunc: func(path string) (string, error) {
					return "/dev/sda", nil
				},
			},
		}

		isAttached, err := m.IsAttached("")
		if err != nil {
			t.Errorf("expected no error: %s", err)
		}

		if isAttached {
			t.Errorf("IsAttached() must return false when the state file does not contains the content: %s\n", runningState)
		}
	})

	t.Run("state file content indicates a running state", func(t *testing.T) {
		m := &mounter{
			log:      logrus.NewEntry(logrus.New()),
			kMounter: nil,
			attachmentValidator: &testAttachmentValidator{
				readFileFunc: func(name string) ([]byte, error) {
					return []byte(fmt.Sprintf("%s\n", runningState)), nil
				},
				evalSymlinksFunc: func(path string) (string, error) {
					return "/dev/sda", nil
				},
			},
		}

		isAttached, err := m.IsAttached("")
		if err != nil {
			t.Errorf("expected no error: %s", err)
		}

		if !isAttached {
			t.Errorf("IsAttached() must return true when the state file does contains the content: %s\n", runningState)
		}
	})
}
