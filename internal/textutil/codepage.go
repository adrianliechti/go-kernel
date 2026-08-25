// Package textutil contains shared character-decoding helpers used by the
// document extractors.
package textutil

import (
	"bytes"
	"encoding/binary"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// DecodeCodePage decodes bytes using a Windows or ISO code-page number.
// Unknown code pages fall back to Windows-1252, matching Office's default.
func DecodeCodePage(data []byte, codePage int) string {
	if codePage == 1200 {
		return decodeUTF16(data, binary.LittleEndian)
	}
	if codePage == 1201 {
		return decodeUTF16(data, binary.BigEndian)
	}
	data = bytes.TrimRight(data, "\x00")
	if codePage == 65001 || codePage == 20127 || utf8.Valid(data) {
		return strings.TrimRight(strings.ToValidUTF8(string(data), "\ufffd"), "\x00")
	}
	enc := encodingForCodePage(codePage)
	if enc == nil {
		enc = charmap.Windows1252
	}
	decoded, _, err := transform.Bytes(enc.NewDecoder(), data)
	if err != nil {
		return strings.ToValidUTF8(string(data), "\ufffd")
	}
	return strings.TrimRight(string(decoded), "\x00")
}

func decodeUTF16(data []byte, order binary.ByteOrder) string {
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = order.Uint16(data[i*2 : i*2+2])
	}
	return strings.TrimRight(string(utf16.Decode(units)), "\x00")
}

func encodingForCodePage(codePage int) encoding.Encoding {
	switch codePage {
	case 37:
		return charmap.CodePage037
	case 437:
		return charmap.CodePage437
	case 850:
		return charmap.CodePage850
	case 852:
		return charmap.CodePage852
	case 855:
		return charmap.CodePage855
	case 858:
		return charmap.CodePage858
	case 860:
		return charmap.CodePage860
	case 862:
		return charmap.CodePage862
	case 863:
		return charmap.CodePage863
	case 865:
		return charmap.CodePage865
	case 866:
		return charmap.CodePage866
	case 874:
		return charmap.Windows874
	case 932:
		return japanese.ShiftJIS
	case 936, 54936:
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
	case 28591:
		return charmap.ISO8859_1
	case 28592:
		return charmap.ISO8859_2
	case 28593:
		return charmap.ISO8859_3
	case 28594:
		return charmap.ISO8859_4
	case 28595:
		return charmap.ISO8859_5
	case 28596:
		return charmap.ISO8859_6
	case 28597:
		return charmap.ISO8859_7
	case 28598:
		return charmap.ISO8859_8
	case 28599:
		return charmap.ISO8859_9
	default:
		return nil
	}
}
