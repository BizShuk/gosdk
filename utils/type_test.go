package utils

import "testing"

func TestIsNil(t *testing.T) {
	type args struct {
		p interface{}
	}
	var nilString *string
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Is nil",
			args: args{
				p: nil,
			},
			want: true,
		},
		{
			name: "string pointer Is Not nil",
			args: args{
				p: StringPointer("abcd"),
			},
			want: false,
		},
		{
			name: "nil string pointer Is Not nil",
			args: args{
				p: nilString,
			},
			want: true,
		},
		{
			name: "int Is Not nil",
			args: args{
				p: 3,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNil(tt.args.p); got != tt.want {
				t.Errorf("IsNil() = %v, want %v", got, tt.want)
			}
		})
	}
}
