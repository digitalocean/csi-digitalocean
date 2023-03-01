package driver

import (
	"errors"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"testing"
)

func Test_mounter_IsRunning(t *testing.T) {
	type args struct {
		source string
	}
	tests := []struct {
		name   string
		args   args
		expect func(*MockiAttachmentValidator)
		want   bool
	}{
		{
			name: "could not evaluate the symbolic link",
			args: args{
				source: "my-source",
			},
			expect: func(av *MockiAttachmentValidator) {
				av.EXPECT().evalSymlinks(gomock.Any()).Times(1).Return("", errors.New("error"))
			},
			want: false,
		},
		{
			name: "error reading the device state file",
			args: args{
				source: "my-source",
			},
			expect: func(av *MockiAttachmentValidator) {
				av.EXPECT().evalSymlinks(gomock.Any()).Times(1).Return("/dev/sda", nil)
				av.EXPECT().readFile(gomock.Any()).Times(1).Return(nil, errors.New("error"))
			},
			want: false,
		},
		{
			name: "state file content does not indicate a running state",
			args: args{
				source: "my-source",
			},
			expect: func(av *MockiAttachmentValidator) {
				av.EXPECT().evalSymlinks(gomock.Any()).Times(1).Return("/dev/sda", nil)
				av.EXPECT().readFile(gomock.Any()).Times(1).Return([]byte("not-running\n"), nil)
			},
			want: false,
		},
		{
			name: "state file content indicates a running state",
			args: args{
				source: "my-source",
			},
			expect: func(av *MockiAttachmentValidator) {
				av.EXPECT().evalSymlinks(gomock.Any()).Times(1).Return("/dev/sda", nil)
				av.EXPECT().readFile(gomock.Any()).Times(1).Return([]byte(runningState+"\n"), nil)
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

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			av := NewMockiAttachmentValidator(ctrl)

			tt.expect(av)

			if got := m.IsRunning(av, tt.args.source); got != tt.want {
				t.Errorf("IsRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}
