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
			name:  "ssh command with name",
			input: []string{"ssh", "test-1"},
			want: invocation{
				command: "ssh",
				args:    []string{"test-1"},
			},
		},
		{
			name:    "ssh without name",
			input:   []string{"ssh"},
			wantErr: true,
		},
		{
			name:    "ssh with extra argument",
			input:   []string{"ssh", "test-1", "extra"},
			wantErr: true,
		},
		{
			name:  "start command with name",
			input: []string{"start", "test-1"},
			want: invocation{
				command: "update",
				args:    []string{"start", "test-1"},
			},
		},
		{
			name:    "start without name",
			input:   []string{"start"},
			wantErr: true,
		},
		{
			name:    "start with extra argument",
			input:   []string{"start", "test-1", "extra"},
			wantErr: true,
		},
		{
			name:  "stop command with name",
			input: []string{"stop", "test-1"},
			want: invocation{
				command: "update",
				args:    []string{"stop", "test-1"},
			},
		},
		{
			name:    "stop without name",
			input:   []string{"stop"},
			wantErr: true,
		},
		{
			name:    "stop with extra argument",
			input:   []string{"stop", "test-1", "extra"},
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
