package runner

import "testing"

func TestIsSimpleCommand(t *testing.T) {
	tests := []struct {
		cmd    string
		simple bool
	}{
		// Simple commands — should get auto-exec
		{"./serve", true},
		{"/usr/bin/daemon -c cfg.yaml", true},
		{"ENV=val ./serve --port 8080", true},
		{"MY_VAR=hello OTHER=world ./cmd", true},
		{`./serve "hello world"`, true},
		{"./serve 'single quotes'", true},
		{"./serve --flag=value", true},
		{"  ./serve  ", true},
		{"/bin/sleep 10", true},
		{"python3 -m http.server 8000", true},
		{"MY_ENV=hello ./test/_helpers/bgserver/bgserver --env MY_ENV", true},

		// Complex commands — no auto-exec
		{"cmd1 | cmd2", false},
		{"cmd1 || cmd2", false},
		{"cmd1 && cmd2", false},
		{"cmd ; cmd2", false},
		{"./serve > /dev/null", false},
		{"./serve >> log", false},
		{"./serve < input", false},
		{"./serve 2>&1", false},
		{"VAR=$(cmd) ./serve", false},
		{"`cmd` ./serve", false},
		{"{ cmd1; cmd2; }", false},
		{"(cmd1)", false},
		{"echo $HOME", false},
		{"./serve > /dev/null 2>&1", false},
		{"cd dir && ./serve", false},
		{"cat file | ./serve", false},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			got := isSimpleCommand(tt.cmd)
			if got != tt.simple {
				t.Errorf("isSimpleCommand(%q) = %v, want %v", tt.cmd, got, tt.simple)
			}
		})
	}
}

func TestMaybeExec(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Simple → prepend exec
		{"./serve", "exec ./serve"},
		{"/usr/bin/daemon --port 80", "exec /usr/bin/daemon --port 80"},

		// Env prefix → exec inserted after env vars
		{"ENV=val ./serve", "ENV=val exec ./serve"},
		{"MY_VAR=hello OTHER=world ./cmd --flag", "MY_VAR=hello OTHER=world exec ./cmd --flag"},

		// Already has exec → no change
		{"exec ./serve", "exec ./serve"},
		{"  exec ./serve", "  exec ./serve"},

		// Complex → no change
		{"cmd1 | cmd2", "cmd1 | cmd2"},
		{"cmd && cmd2", "cmd && cmd2"},
		{"./serve > log", "./serve > log"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := maybeExec(tt.input)
			if got != tt.expected {
				t.Errorf("maybeExec(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
