package dataset

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitText(t *testing.T) {
	short := "fits as is"
	if got := splitText(short, 100); len(got) != 1 || got[0] != short {
		t.Errorf("short text: got %q", got)
	}

	long := strings.Repeat("A sentence here. ", 20) + "\n" + strings.Repeat("word ", 50)
	pieces := splitText(long, 60)
	if len(pieces) < 2 {
		t.Fatalf("long text: got %d pieces", len(pieces))
	}
	for _, p := range pieces {
		if n := utf8.RuneCountInString(p); n > 60 {
			t.Errorf("piece over max (%d): %q", n, p)
		}
	}
	if joined := strings.Join(strings.Fields(strings.Join(pieces, " ")), " "); joined != strings.Join(strings.Fields(long), " ") {
		t.Error("pieces lost or reordered text")
	}
}

func TestStripJSON(t *testing.T) {
	text := "Cycle the towpath.\n{\"type\": \"Feature\",\n\"geometry\": {\"coordinates\": [[1,2],[3,4]]}}\n{not json} stays.\n"
	want := "Cycle the towpath.\n\n{not json} stays.\n"
	if got := stripJSON(text); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
