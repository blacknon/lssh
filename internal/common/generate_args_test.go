package common

import (
	"reflect"
	"testing"
)

func TestNormalizeGenerateLSSHConfArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "default path",
			args: []string{"cmd", "--generate-lssh-conf"},
			want: []string{"cmd", "--generate-lssh-conf=~/.ssh/config"},
		},
		{
			name: "explicit path",
			args: []string{"cmd", "--generate-lssh-conf=/tmp/ssh_config"},
			want: []string{"cmd", "--generate-lssh-conf=/tmp/ssh_config"},
		},
		{
			name: "other args unchanged",
			args: []string{"cmd", "-H", "app"},
			want: []string{"cmd", "-H", "app"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := NormalizeGenerateLSSHConfArgs(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NormalizeGenerateLSSHConfArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
