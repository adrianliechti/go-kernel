package msg

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/abemedia/go-cfb"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	unicodeencoding "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const (
	ptI2      = 0x0002
	ptLong    = 0x0003
	ptBoolean = 0x000b
	ptI8      = 0x0014
	ptString8 = 0x001e
	ptUnicode = 0x001f
	ptSysTime = 0x0040
	ptBinary  = 0x0102
)

type mapiObject struct {
	storage    *cfb.Storage
	streams    map[uint32][]byte
	fixed      map[uint32]uint64
	codePage   int
	streamSize int64
}

func readMAPIObject(storage *cfb.Storage, messageObject bool, maxStream int64) (*mapiObject, error) {
	o := &mapiObject{
		storage:    storage,
		streams:    make(map[uint32][]byte),
		fixed:      make(map[uint32]uint64),
		codePage:   1252,
		streamSize: maxStream,
	}
	var properties []byte
	for _, entry := range storage.Entries {
		stream, ok := entry.(*cfb.Stream)
		if !ok {
			continue
		}
		if stream.Size < 0 || stream.Size > maxStream {
			return nil, fmt.Errorf("%w: MSG stream %q is too large", ErrResourceLimit, stream.Name)
		}
		data, err := io.ReadAll(io.LimitReader(stream.Open(), maxStream+1))
		if err != nil {
			return nil, fmt.Errorf("msg: read MSG stream %q: %w", stream.Name, err)
		}
		if int64(len(data)) > maxStream {
			return nil, fmt.Errorf("%w: MSG stream %q is too large", ErrResourceLimit, stream.Name)
		}
		if strings.EqualFold(stream.Name, "__properties_version1.0") {
			properties = data
			continue
		}
		if tag, ok := streamPropertyTag(stream.Name); ok {
			o.streams[tag] = data
		}
	}

	headerSize := 8
	if messageObject {
		headerSize = 32
	}
	if len(properties) >= headerSize {
		for offset := headerSize; offset+16 <= len(properties); offset += 16 {
			tag := binary.LittleEndian.Uint32(properties[offset : offset+4])
			typeID := uint16(tag)
			switch typeID {
			case ptI2:
				o.fixed[tag] = uint64(binary.LittleEndian.Uint16(properties[offset+8 : offset+10]))
			case ptLong:
				o.fixed[tag] = uint64(binary.LittleEndian.Uint32(properties[offset+8 : offset+12]))
			case ptBoolean:
				o.fixed[tag] = uint64(binary.LittleEndian.Uint16(properties[offset+8 : offset+10]))
			case ptI8, ptSysTime:
				o.fixed[tag] = binary.LittleEndian.Uint64(properties[offset+8 : offset+16])
			}
		}
	}
	for _, id := range []uint16{0x3fde, 0x3ffd} {
		if value, ok := o.integer(id); ok && value > 0 && value <= 65535 {
			o.codePage = int(value)
			break
		}
	}
	return o, nil
}

func streamPropertyTag(name string) (uint32, bool) {
	const prefix = "__substg1.0_"
	if len(name) < len(prefix)+8 || !strings.EqualFold(name[:len(prefix)], prefix) {
		return 0, false
	}
	value, err := strconv.ParseUint(name[len(prefix):len(prefix)+8], 16, 32)
	if err != nil {
		return 0, false
	}
	return uint32(value), true
}

func propertyTag(id, typeID uint16) uint32 {
	return uint32(id)<<16 | uint32(typeID)
}

func (o *mapiObject) string(id uint16) string {
	if data, ok := o.streams[propertyTag(id, ptUnicode)]; ok {
		return decodeUTF16LE(data)
	}
	if data, ok := o.streams[propertyTag(id, ptString8)]; ok {
		return decodeCodePage(data, o.codePage)
	}
	return ""
}

func (o *mapiObject) binary(id uint16) []byte {
	return o.streams[propertyTag(id, ptBinary)]
}

func (o *mapiObject) integer(id uint16) (uint64, bool) {
	for _, typeID := range []uint16{ptLong, ptI2, ptBoolean, ptI8} {
		if value, ok := o.fixed[propertyTag(id, typeID)]; ok {
			return value, true
		}
	}
	return 0, false
}

func (o *mapiObject) boolean(id uint16) bool {
	value, ok := o.fixed[propertyTag(id, ptBoolean)]
	return ok && value != 0
}

func (o *mapiObject) time(id uint16) time.Time {
	value, ok := o.fixed[propertyTag(id, ptSysTime)]
	if !ok || value == 0 {
		return time.Time{}
	}
	return filetime(value)
}

func (o *mapiObject) childStorages(prefix string) []*cfb.Storage {
	var result []*cfb.Storage
	for _, entry := range o.storage.Entries {
		if storage, ok := entry.(*cfb.Storage); ok && strings.HasPrefix(strings.ToLower(storage.Name), strings.ToLower(prefix)) {
			result = append(result, storage)
		}
	}
	return result
}

func (o *mapiObject) childStorage(name string) *cfb.Storage {
	for _, entry := range o.storage.Entries {
		if storage, ok := entry.(*cfb.Storage); ok && strings.EqualFold(storage.Name, name) {
			return storage
		}
	}
	return nil
}

func decodeUTF16LE(data []byte) string {
	data = bytes.TrimSuffix(data, []byte{0, 0})
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
	}
	return strings.TrimRight(string(utf16.Decode(units)), "\x00")
}

func decodeCodePage(data []byte, codePage int) string {
	data = bytes.TrimRight(data, "\x00")
	if codePage == 1200 {
		return decodeUTF16LE(data)
	}
	if codePage == 1201 {
		decoder := unicodeencoding.UTF16(unicodeencoding.BigEndian, unicodeencoding.IgnoreBOM).NewDecoder()
		if decoded, _, err := transform.Bytes(decoder, data); err == nil {
			return strings.TrimRight(string(decoded), "\x00")
		}
	}
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
	}
	return nil
}

func filetime(ticks uint64) time.Time {
	const unixEpoch = uint64(116444736000000000)
	if ticks < unixEpoch {
		return time.Time{}
	}
	ticks -= unixEpoch
	seconds := int64(ticks / 10_000_000)
	nanos := int64(ticks%10_000_000) * 100
	return time.Unix(seconds, nanos).UTC()
}
