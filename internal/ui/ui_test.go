package ui

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// fixedTime returns a UI bound to a deterministic clock, so log
// timestamps are stable across runs.
func fixedTime(mode Mode, now time.Time) (*UI, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	err := &bytes.Buffer{}
	u := New(mode, out, err)
	u.now = func() time.Time { return now }
	return u, out, err
}

func TestParseMode(t *testing.T) {
	cases := map[string]Mode{
		"":      ModeAuto,
		"auto":  ModeAuto,
		"tui":   ModeTUI,
		"log":   ModeLog,
		"bogus": ModeAuto,
	}
	for in, want := range cases {
		if got := ParseMode(in); got != want {
			t.Errorf("ParseMode(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestLogModeFormatsTimestamps(t *testing.T) {
	u, out, _ := fixedTime(ModeLog, time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC))
	u.Step("resolving %s", "PowerShell 7.5.6")

	// Log mode keeps the line plain — no Unicode arrow prefix; the
	// timestamp + level pair already serves that purpose.
	// Single space between columns; first letter capitalised.
	got := out.String()
	want := "2026-05-10T12:00:00Z INFO Resolving PowerShell 7.5.6\n"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestLogModeWarnAndErrorGoToStderr(t *testing.T) {
	u, out, errBuf := fixedTime(ModeLog, time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC))
	u.Warn("uh oh")
	u.Errorf("nope")

	if out.Len() != 0 {
		t.Errorf("warn/error should not write to stdout, got %q", out.String())
	}
	if !strings.Contains(errBuf.String(), "WARN") {
		t.Errorf("warn line missing WARN label: %q", errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "ERROR") {
		t.Errorf("error line missing ERROR label: %q", errBuf.String())
	}
}

func TestTUIStepStartsSpinnerThenDoneFinalises(t *testing.T) {
	u, out, _ := fixedTime(ModeTUI, time.Now())

	u.Step("hi")
	stepOut := out.String()
	if !strings.Contains(stepOut, ansiClearLine) {
		t.Errorf("Step should have cleared the line first, got %q", stepOut)
	}
	if !strings.Contains(stepOut, spinFrames[0]) {
		t.Errorf("Step should have drawn the first spinner frame, got %q", stepOut)
	}

	out.Reset()
	u.Done("hi done")
	doneOut := out.String()
	if !strings.Contains(doneOut, "✓") {
		t.Errorf("Done should have written the ✓ checkmark, got %q", doneOut)
	}
	if !strings.Contains(doneOut, "Hi done") {
		t.Errorf("Done message should be capitalised, got %q", doneOut)
	}
}

func TestDownloadWriteCountsBytes(t *testing.T) {
	u, _, _ := fixedTime(ModeLog, time.Now())
	d := u.BeginDownload("file.zip", 1000)

	n, err := d.Write([]byte("hello world!"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 12 {
		t.Errorf("Write returned %d, want 12", n)
	}
	if d.written != 12 {
		t.Errorf("d.written = %d, want 12", d.written)
	}
}

func TestDownloadDoneIsIdempotent(t *testing.T) {
	u, out, _ := fixedTime(ModeLog, time.Now())
	d := u.BeginDownload("file.zip", 1000)
	d.Write([]byte("data"))
	d.Done()
	d.Done() // second call must not panic or print twice

	got := out.String()
	if strings.Count(got, "Downloaded") != 1 {
		t.Errorf("expected exactly one 'Downloaded' line, got: %q", got)
	}
}

func TestBannerOnlyInTUIMode(t *testing.T) {
	// Log mode: no banner, ever — would noise up CI output.
	logUI, logOut, _ := fixedTime(ModeLog, time.Now())
	logUI.Banner()
	if logOut.Len() != 0 {
		t.Errorf("log mode wrote banner output: %q", logOut.String())
	}

	// TUI mode: banner emitted with brand colour and surrounding blank
	// lines.
	tuiUI, tuiOut, _ := fixedTime(ModeTUI, time.Now())
	tuiUI.Banner()
	got := tuiOut.String()

	if !strings.Contains(got, conchColour) {
		t.Errorf("expected brand colour escape %q, got %q", conchColour, got)
	}
	if !strings.Contains(got, "██████╗") {
		t.Errorf("expected banner glyphs in output, got %q", got)
	}
	if !strings.HasPrefix(got, "\n") {
		t.Errorf("banner must start with a newline, got %q", got[:min(20, len(got))])
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Errorf("banner must end with a trailing blank line")
	}
}

func TestFmtBytes(t *testing.T) {
	cases := map[int64]string{
		0:                      "0 B",
		512:                    "512 B",
		1024:                   "1.0 KB",
		1500 * 1024:            "1.5 MB",
		3 * 1024 * 1024 * 1024: "3.0 GB",
		-1:                     "?",
	}
	for in, want := range cases {
		if got := fmtBytes(in); got != want {
			t.Errorf("fmtBytes(%d) = %q, want %q", in, got, want)
		}
	}
}
