package driver

import "testing"

func Test_mounter_IsRunning(t *testing.T) {
	type fields struct {
		log      *logrus.Entry
		kMounter *mount.SafeFormatAndMount
	}
	type args struct {
		source string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mounter{
				log:      tt.fields.log,
				kMounter: tt.fields.kMounter,
			}
			if got := m.IsRunning(tt.args.source); got != tt.want {
				t.Errorf("IsRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}
