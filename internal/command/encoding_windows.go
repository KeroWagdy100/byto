//go:build windows

package command

import (
	"syscall"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

var acpDecoder *encoding.Decoder

func init() {
	acp := getActiveCodePage()
	enc := codepageToEncoding(acp)
	if enc != nil {
		acpDecoder = enc.NewDecoder()
	}
}

func getActiveCodePage() uint32 {
	ret, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetACP").Call()
	return uint32(ret)
}

func codepageToEncoding(cp uint32) encoding.Encoding {
	switch cp {
	case 874:
		return charmap.Windows874
	case 932:
		return japanese.ShiftJIS
	case 936:
		return simplifiedchinese.GBK
	case 949:
		return korean.EUCKR
	case 950:
		return traditionalchinese.Big5
	case 1250:
		return charmap.Windows1250
	case 1251:
		return charmap.Windows1251
	case 1252:
		return charmap.Windows1252
	case 1253:
		return charmap.Windows1253
	case 1254:
		return charmap.Windows1254
	case 1255:
		return charmap.Windows1255
	case 1256:
		return charmap.Windows1256
	case 1257:
		return charmap.Windows1257
	case 1258:
		return charmap.Windows1258
	case 65001:
		return nil // already UTF-8
	default:
		return nil
	}
}

// ensureUTF8 checks if s is valid UTF-8. If not, decodes from the system code page.
// utf8.ValidString is a zero-alloc byte scan (~150ns for a typical line).
// Actual decoding only happens on non-UTF-8 lines.
func ensureUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	if acpDecoder == nil {
		return s
	}
	decoded, err := acpDecoder.String(s)
	if err != nil {
		return s
	}
	return decoded
}
