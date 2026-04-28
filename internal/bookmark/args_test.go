package bookmark

import "testing"

func TestParseArgsPreservesSingleQuotedWindowsPath(t *testing.T) {
	t.Parallel()

	args, err := ParseArgs("-mm 'C:\\models\\Vision Path\\mmproj.gguf'")
	if err != nil {
		t.Fatalf("ParseArgs returned error: %v", err)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if got, want := args[1], "C:\\models\\Vision Path\\mmproj.gguf"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
