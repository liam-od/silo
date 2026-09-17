package main

import (
	"slices"
	"testing"
)

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    invocation
		wantErr bool
	}{
		{
			name:  "no arguments selects info",
			input: nil,
			want:  invocation{command: "info"},
		},
		{
			name:  "list command",
			input: []string{"list"},
			want:  invocation{command: "list"},
		},
		{
			name:  "create command with name",
			input: []string{"create", "test-1"},
			want: invocation{
				command: "create",
				args:    []string{"test-1"},
			},
		},
		{
			name:    "create without name",
			input:   []string{"create"},
			wantErr: true,
		},
		{
			name:    "create with extra argument",
			input:   []string{"create", "test-1", "extra"},
			wantErr: true,
		},
		{
			name:    "list with extra argument",
			input:   []string{"list", "extra"},
			wantErr: true,
		},
		{
			name:    "unknown command",
			input:   []string{"unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgs() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if got.command != tt.want.command {
				t.Errorf("command = %q, want %q", got.command, tt.want.command)
			}

			if !slices.Equal(got.args, tt.want.args) {
				t.Errorf("args = %v, want %v", got.args, tt.want.args)
			}
		})
	}
}
