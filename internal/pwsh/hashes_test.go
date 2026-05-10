package pwsh

import (
	"bytes"
	"testing"
)

// utf16LEWithBOM encodes ASCII text the way PowerShell's hashes.sha256
// is published — UTF-16 LE with a leading BOM. Used to reproduce the
// exact byte stream the prototype had to defeat.
func utf16LEWithBOM(s string) []byte {
	var b bytes.Buffer
	b.Write([]byte{0xFF, 0xFE})
	for _, r := range s {
		b.WriteByte(byte(r & 0xFF))
		b.WriteByte(byte(r >> 8))
	}
	return b.Bytes()
}

func TestDecodeHashesFileUTF16LE(t *testing.T) {
	// Mirrors the real GitHub release file: sha256sum binary-mode
	// output ("<hash> *<filename>"), encoded as UTF-16 LE with a BOM,
	// using PowerShell's actual mixed-case naming (PascalCase for
	// Windows, lowercase for Linux).
	body := utf16LEWithBOM(
		"6ce82f1b7438d0943a04043b118e1b0b70e54593ce07310094276effb64c5e9c *PowerShell-7.5.6-win-x64.zip\n" +
			"9b19464014bac0e007d10a99cf858fc4ca3f4e62c3c8ca2b01c51dd33e867434 *powershell-7.5.6-linux-x64.tar.gz\n",
	)
	got, err := decodeHashesFile(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got["PowerShell-7.5.6-win-x64.zip"] != "6ce82f1b7438d0943a04043b118e1b0b70e54593ce07310094276effb64c5e9c" {
		t.Errorf("zip hash = %q", got["PowerShell-7.5.6-win-x64.zip"])
	}
	if got["powershell-7.5.6-linux-x64.tar.gz"] != "9b19464014bac0e007d10a99cf858fc4ca3f4e62c3c8ca2b01c51dd33e867434" {
		t.Errorf("tar.gz hash = %q", got["powershell-7.5.6-linux-x64.tar.gz"])
	}
}

func TestDecodeHashesFilePlainUTF8(t *testing.T) {
	body := []byte("AABBCC  file.zip\n")
	got, err := decodeHashesFile(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if got["file.zip"] != "aabbcc" {
		t.Errorf("got %v", got)
	}
}
