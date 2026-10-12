package markedblock

import (
	"bytes"
	"testing"
)

func TestReplace(t *testing.T) {
	const begin, end = "// generated begin", "// generated end"
	for _, test := range []struct {
		name, current, body, want string
		invalid                   bool
	}{
		{name: "append", current: "manual", body: "setting\n", want: "manual\n// generated begin\nsetting\n// generated end\n"},
		{name: "move last", current: "before\n// generated begin\nold\n// generated end\nafter\n", body: "new\n", want: "before\nafter\n// generated begin\nnew\n// generated end\n"},
		{name: "clear CRLF", current: "before\r\n// generated begin\r\nold\r\n// generated end\r\nafter\r\n", want: "before\r\nafter\r\n"},
		{name: "marker in string", current: "user_pref(\"example\", \"// generated begin\");\n", want: "user_pref(\"example\", \"// generated begin\");\n"},
		{name: "unterminated", current: "// generated begin\n", invalid: true},
		{name: "reversed", current: "// generated end\n// generated begin\n", invalid: true},
		{name: "duplicate", current: "// generated begin\n// generated end\n// generated begin\n// generated end\n", invalid: true},
		{name: "begin suffix", current: "// generated begin junk\n// generated end\n", invalid: true},
		{name: "end suffix", current: "// generated begin\n// generated end\rjunk\n", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := Replace([]byte(test.current), begin, end, []byte(test.body))
			if (err != nil) != test.invalid {
				t.Fatalf("Replace error = %v", err)
			}
			if test.invalid {
				return
			}
			if string(got) != test.want {
				t.Fatalf("Replace = %q, want %q", got, test.want)
			}
			again, err := Replace(got, begin, end, []byte(test.body))
			if err != nil || !bytes.Equal(got, again) {
				t.Fatalf("second replacement changed result: %q, %v", again, err)
			}
		})
	}
}
