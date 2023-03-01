package driver

import (
	"errors"
	"github.com/sirupsen/logrus"
	"testing"
)

func Test_mounter_IsRunning(t *testing.T) {
	type args struct {
		source string
	}
	tests := []struct {
		name         string
		args         args
		evalSymlinks func(path string) (string, error)
		readFileFunc func(name string) ([]byte, error)
		want         bool
	}{
		{
			name: "could not evaluate the symbolic link",
			args: args{
				source: "my-source",
			},
			evalSymlinks: func(path string) (string, error) {
				return "", errors.New("error")
			},
			readFileFunc: nil,
			want:         false,
		},
		{
			name: "error reading the device state file",
			args: args{
				source: "my-source",
			},
			evalSymlinks: func(path string) (string, error) {
				return "/dev/sda", nil
			},
			readFileFunc: func(name string) ([]byte, error) {
				return nil, errors.New("error")
			},
			want: false,
		},
		{
			name: "state file content does not indicate a running state",
			args: args{
				source: "my-source",
			},
			evalSymlinks: func(path string) (string, error) {
				return "/dev/sda", nil
			},
			readFileFunc: func(name string) ([]byte, error) {
				return []byte("not-running\n"), nil
			},
			want: false,
		},
		{
			name: "state file content indicates a running state",
			args: args{
				source: "my-source",
			},
			evalSymlinks: func(path string) (string, error) {
				return "/dev/sda", nil
			},
			readFileFunc: func(name string) ([]byte, error) {
				return []byte(runningState + "\n"), nil
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mounter{
				log:      logrus.NewEntry(logrus.New()),
				kMounter: nil,
			}

			readFile = tt.readFileFunc
			evalSymlinks = tt.evalSymlinks

			if got := m.IsRunning(tt.args.source); got != tt.want {
				t.Errorf("IsRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}
