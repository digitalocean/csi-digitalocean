package driver

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/sirupsen/logrus"
	"k8s.io/mount-utils"
)

const (
	testRetrieveMountNameError      = "TestRetrieveMountNameError"
	testRetrieveRunningStateError   = "TestRetrieveRunningStateError"
	testRetrieveRunningStateSuccess = "TestRetrieveRunningStateSuccess"
)

func TestRetrieveMountNameError(t *testing.T) {
	if os.Getenv("GO_TEST_PROCESS") != "1" {
		return
	}

	if _, err := fmt.Fprintf(os.Stdout, "error"); err != nil {
		panic("error when writing to stdout")
	}

	os.Exit(1)
}

func TestRetrieveMountNameSuccess(t *testing.T) {
	if os.Getenv("GO_TEST_PROCESS") != "1" {
		return
	}

	testName := os.Getenv("GO_TEST_NAME")
	if testName == "" {
		panic("error GO_TEST_NAME must be specified")
	}

	if _, err := fmt.Fprintf(os.Stdout, "../../sd%s\n", testName); err != nil {
		panic("error when writing to stdout")
	}

	os.Exit(0)
}

func TestRetrieveRunningStateError(t *testing.T) {
	if os.Getenv("GO_TEST_PROCESS") != "1" {
		return
	}

	if _, err := fmt.Fprintf(os.Stdout, "0\n"); err != nil {
		panic("error when writing to stdout")
	}

	os.Exit(1)
}

func TestRetrieveRunningStateSuccess(t *testing.T) {
	if os.Getenv("GO_TEST_PROCESS") != "1" {
		return
	}

	if _, err := fmt.Fprintf(os.Stdout, "1\n"); err != nil {
		panic("error when writing to stdout")
	}

	os.Exit(0)
}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	var cs []string
	var extraEnvs []string

	for _, arg := range args {
		if arg == fmt.Sprintf("ls -l %s | awk '{print $NF}'", testRetrieveMountNameError) {
			cs = []string{"-test.run=TestRetrieveMountNameError", "--", command}
		}

		if arg == fmt.Sprintf("ls -l %s | awk '{print $NF}'", testRetrieveRunningStateError) {
			cs = []string{"-test.run=TestRetrieveMountNameSuccess", "--", command}
			extraEnvs = append(extraEnvs, fmt.Sprintf("GO_TEST_NAME=%s", testRetrieveRunningStateError))
		}
		if arg == fmt.Sprintf("cat /sys/class/block/sd%s/device/state | grep -x running | wc -l", testRetrieveRunningStateError) {
			cs = []string{"-test.run=TestRetrieveRunningStateError", "--", command}
		}

		if arg == fmt.Sprintf("ls -l %s | awk '{print $NF}'", testRetrieveRunningStateSuccess) {
			cs = []string{"-test.run=TestRetrieveMountNameSuccess", "--", command}
			extraEnvs = append(extraEnvs, fmt.Sprintf("GO_TEST_NAME=%s", testRetrieveRunningStateSuccess))
		}
		if arg == fmt.Sprintf("cat /sys/class/block/sd%s/device/state | grep -x running | wc -l", testRetrieveRunningStateSuccess) {
			cs = []string{"-test.run=TestRetrieveRunningStateSuccess", "--", command}
		}
	}

	if len(cs) == 0 {
		panic("could not set a valid fake command")
	}

	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{
		"GO_TEST_PROCESS=1",
	}
	cmd.Env = append(cmd.Env, extraEnvs...)

	return cmd
}

func Test_mounter_IsRunning(t *testing.T) {
	type fields struct {
		log      *logrus.Entry
		kMounter *mount.SafeFormatAndMount
	}
	type args struct {
		source     string
		cmdContext execContext
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "error retrieving the mount name",
			fields: fields{
				log: logrus.NewEntry(logrus.New()),
			},
			args: args{
				source:     testRetrieveMountNameError,
				cmdContext: fakeExecCommand,
			},
			want: false,
		},
		{
			name: "error retrieving the running state",
			fields: fields{
				log: logrus.NewEntry(logrus.New()),
			},
			args: args{
				source:     testRetrieveRunningStateError,
				cmdContext: fakeExecCommand,
			},
			want: false,
		},
		{
			name: "success path",
			fields: fields{
				log: logrus.NewEntry(logrus.New()),
			},
			args: args{
				source:     testRetrieveRunningStateSuccess,
				cmdContext: fakeExecCommand,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mounter{
				log:      tt.fields.log,
				kMounter: tt.fields.kMounter,
			}
			if got := m.IsRunning(tt.args.source, tt.args.cmdContext); got != tt.want {
				t.Errorf("IsRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}
