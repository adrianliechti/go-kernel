package msg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	rtfMagicCompressed   = 0x75465a4c // LZFu
	rtfMagicUncompressed = 0x414c454d // MELA
)

var rtfDictionary = []byte("{\\rtf1\\ansi\\mac\\deff0\\deftab720{\\fonttbl;}{\\f0\\fnil \\froman \\fswiss \\fmodern \\fscript \\fdecor MS Sans SerifSymbolArialTimes New RomanCourier{\\colortbl\\red0\\green0\\blue0\r\n\\par \\pard\\plain\\f0\\fs20\\b\\i\\u\\tab\\tx")

func decompressRTF(data []byte) ([]byte, error) {
	if len(data) < 16 {
		return nil, errors.New("msg: compressed RTF header is truncated")
	}
	compressedSize := binary.LittleEndian.Uint32(data[0:4])
	rawSize := binary.LittleEndian.Uint32(data[4:8])
	magic := binary.LittleEndian.Uint32(data[8:12])
	if rawSize > 512<<20 || uint64(rawSize) > uint64(len(data))*16+1<<20 {
		return nil, errors.New("msg: compressed RTF expands beyond safety limit")
	}
	payload := data[16:]
	if compressedSize >= 12 && int(compressedSize-12) < len(payload) {
		payload = payload[:compressedSize-12]
	}
	if magic == rtfMagicUncompressed {
		if int(rawSize) < len(payload) {
			payload = payload[:rawSize]
		}
		return append([]byte(nil), payload...), nil
	}
	if magic != rtfMagicCompressed {
		return nil, errors.New("msg: unknown compressed RTF magic")
	}

	dictionary := make([]byte, 4096)
	copy(dictionary, rtfDictionary)
	writePos := len(rtfDictionary) & 0x0fff
	result := make([]byte, 0, rawSize)
	for pos := 0; pos < len(payload) && len(result) < int(rawSize); {
		flags := payload[pos]
		pos++
		for bit := 0; bit < 8 && pos < len(payload) && len(result) < int(rawSize); bit++ {
			if flags&(1<<bit) == 0 {
				value := payload[pos]
				pos++
				result = append(result, value)
				dictionary[writePos] = value
				writePos = (writePos + 1) & 0x0fff
				continue
			}
			if pos+1 >= len(payload) {
				break
			}
			token := uint16(payload[pos])<<8 | uint16(payload[pos+1])
			pos += 2
			readPos := int(token >> 4)
			length := int(token&0x0f) + 2
			for i := 0; i < length && len(result) < int(rawSize); i++ {
				value := dictionary[readPos]
				readPos = (readPos + 1) & 0x0fff
				result = append(result, value)
				dictionary[writePos] = value
				writePos = (writePos + 1) & 0x0fff
			}
		}
	}
	if len(result) == 0 && rawSize != 0 {
		return nil, errors.New("msg: compressed RTF contains no data")
	}
	return result, nil
}

type rtfState struct {
	skip   bool
	ucSkip int
}

func rtfToText(data []byte, codePage int) string {
	states := []rtfState{{ucSkip: 1}}
	var out strings.Builder
	for i := 0; i < len(data); {
		switch data[i] {
		case '{':
			states = append(states, states[len(states)-1])
			i++
		case '}':
			if len(states) > 1 {
				states = states[:len(states)-1]
			}
			i++
		case '\\':
			i++
			if i >= len(data) {
				break
			}
			if strings.ContainsRune("{}\\", rune(data[i])) {
				if !states[len(states)-1].skip {
					out.WriteByte(data[i])
				}
				i++
				continue
			}
			if data[i] == '*' {
				states[len(states)-1].skip = true
				i++
				continue
			}
			if data[i] == '\'' && i+2 < len(data) {
				if value, err := strconv.ParseUint(string(data[i+1:i+3]), 16, 8); err == nil && !states[len(states)-1].skip {
					out.WriteString(decodeCodePage([]byte{byte(value)}, codePage))
				}
				i += 3
				continue
			}
			start := i
			for i < len(data) && ((data[i] >= 'a' && data[i] <= 'z') || (data[i] >= 'A' && data[i] <= 'Z')) {
				i++
			}
			word := strings.ToLower(string(data[start:i]))
			sign := 1
			if i < len(data) && data[i] == '-' {
				sign = -1
				i++
			}
			numStart := i
			for i < len(data) && data[i] >= '0' && data[i] <= '9' {
				i++
			}
			hasNumber := i > numStart
			number := 0
			if hasNumber {
				number, _ = strconv.Atoi(string(data[numStart:i]))
				number *= sign
			}
			if i < len(data) && data[i] == ' ' {
				i++
			}
			state := &states[len(states)-1]
			switch word {
			case "fonttbl", "colortbl", "stylesheet", "info", "pict", "object", "header", "footer", "generator", "datastore", "themedata", "xmlnstbl":
				state.skip = true
			case "uc":
				if hasNumber && number >= 0 {
					state.ucSkip = number
				}
			case "u":
				if !state.skip && hasNumber {
					value := number
					if value < 0 {
						value += 65536
					}
					out.WriteRune(rune(value))
				}
				for skipped := 0; skipped < state.ucSkip && i < len(data); skipped++ {
					if data[i] == '\\' && i+3 < len(data) && data[i+1] == '\'' {
						i += 4
					} else {
						i++
					}
				}
			case "par", "line":
				if !state.skip {
					out.WriteByte('\n')
				}
			case "tab":
				if !state.skip {
					out.WriteByte('\t')
				}
			case "bullet":
				if !state.skip {
					out.WriteString("•")
				}
			case "emdash":
				if !state.skip {
					out.WriteString("—")
				}
			case "endash":
				if !state.skip {
					out.WriteString("–")
				}
			case "lquote":
				if !state.skip {
					out.WriteString("‘")
				}
			case "rquote":
				if !state.skip {
					out.WriteString("’")
				}
			case "ldblquote":
				if !state.skip {
					out.WriteString("“")
				}
			case "rdblquote":
				if !state.skip {
					out.WriteString("”")
				}
			}
		case '\r', '\n':
			i++
		default:
			start := i
			for i < len(data) && !bytes.ContainsRune([]byte("{}\\\r\n"), rune(data[i])) {
				i++
			}
			if !states[len(states)-1].skip {
				chunk := data[start:i]
				if utf8.Valid(chunk) {
					out.Write(chunk)
				} else {
					out.WriteString(decodeCodePage(chunk, codePage))
				}
			}
		}
	}
	return normalizePlainText(out.String())
}
