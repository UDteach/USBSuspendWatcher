package platform

import "testing"

func TestQuoteArgForShellExecuteParameters(t *testing.T) {
	cases := []struct {
		arg  string
		want string
	}{
		{arg: "", want: `""`},
		{arg: "plain", want: "plain"},
		{arg: `C:\Users\User Name\logs`, want: `"C:\Users\User Name\logs"`},
		{arg: `quote"inside`, want: `"quote\"inside"`},
		{arg: `ends-with-slash\`, want: `ends-with-slash\`},
		{arg: `space and slash\`, want: `"space and slash\\"`},
	}

	for _, tc := range cases {
		if got := quoteArg(tc.arg); got != tc.want {
			t.Fatalf("quoteArg(%q) = %q, want %q", tc.arg, got, tc.want)
		}
	}
}

func TestJoinArgsKeepsSwitchValuePairs(t *testing.T) {
	got := joinArgs([]string{"--log-dir", `C:\Users\User Name\logs`, "--parent-pid", "123"})
	want := `--log-dir "C:\Users\User Name\logs" --parent-pid 123`
	if got != want {
		t.Fatalf("joinArgs() = %q, want %q", got, want)
	}
}
