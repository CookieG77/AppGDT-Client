package handler

import "testing"

func TestSetTask(t *testing.T) {
	content := "Intro\n- [ ] un\n  - [x] deux (imbriquée)\n```\n- [ ] dans du code\n```\n> * [X] citée\n1. [ ] numérotée\r\n- [ ]pas une tâche\n+ [ ]"

	tests := []struct {
		index int
		done  bool
		want  string // the modified line
		ok    bool
	}{
		{0, true, "- [x] un", true},
		{1, false, "  - [ ] deux (imbriquée)", true},
		{2, false, "> * [ ] citée", true}, // the task inside the code block is skipped
		{3, true, "1. [x] numérotée\r", true},
		{4, true, "+ [x]", true}, // "- [ ]pas une tâche" is not a task
		{5, true, "", false},
	}
	for _, tt := range tests {
		got, ok := setTask(content, tt.index, tt.done)
		if ok != tt.ok {
			t.Errorf("setTask(%d) ok = %v, want %v", tt.index, ok, tt.ok)
			continue
		}
		if !ok {
			if got != content {
				t.Errorf("setTask(%d) must not change the content", tt.index)
			}
			continue
		}
		if !containsLine(got, tt.want) {
			t.Errorf("setTask(%d, %v): line %q not found in\n%s", tt.index, tt.done, tt.want, got)
		}
	}
}

func containsLine(text, line string) bool {
	for _, l := range splitLines(text) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	return append(lines, s[start:])
}
