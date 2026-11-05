package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	// BeeNTSeparator separates the opcode, plugin index and API arguments.
	BeeNTSeparator = "#|&BEExNT&|#"
	// BeeNTFieldSeparator separates columns inside list records.
	BeeNTFieldSeparator = "/c.BeExNTs.\\"
	MessageContinue     = 0
	MessageIgnore       = MessageContinue
	MessageIntercept    = 1
)

// BeeNTAPI binds a framework transport to a plugin index. It is safe for
// concurrent use when its transport supports concurrent calls.
type BeeNTAPI struct {
	index     int32
	transport APITransport
}

// NewBeeNTAPI returns the API connection installed by the Worker runtime.
func NewBeeNTAPI() (*BeeNTAPI, error) {
	return NewBeeNTAPIWithTransport(currentPluginIndex(), currentAPITransport())
}

// NewBeeNTAPIWithTransport creates an API connection for an explicit plugin.
func NewBeeNTAPIWithTransport(index int32, transport APITransport) (*BeeNTAPI, error) {
	if index < 0 {
		return nil, errors.New("BeeNT plugin index must be nonnegative")
	}
	if transport == nil {
		return nil, errors.New("BeeNT API transport is not connected")
	}
	v := reflect.ValueOf(transport)
	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface || v.Kind() == reflect.Map || v.Kind() == reflect.Func || v.Kind() == reflect.Slice || v.Kind() == reflect.Chan) && v.IsNil() {
		return nil, errors.New("BeeNT API transport is nil")
	}
	return &BeeNTAPI{index: index, transport: transport}, nil
}

// Call invokes an opcode from the supplied BeeNT reference. Framework business
// failures remain the returned text; errors describe validation/IPC failures.
// Results are kept verbatim so JSON and literal backslashes are not corrupted.
func (api *BeeNTAPI) Call(opcode int, args ...string) (string, error) {
	if api == nil || api.transport == nil {
		return "", errors.New("BeeNT API transport is not connected")
	}
	if opcode < 1 || opcode >= len(apiArgumentCounts) {
		return "", fmt.Errorf("unsupported BeeNT opcode %d", opcode)
	}
	if len(args) != apiArgumentCounts[opcode] {
		return "", fmt.Errorf("BeeNT opcode %d requires %d arguments, got %d", opcode, apiArgumentCounts[opcode], len(args))
	}
	fields := make([]string, 0, len(args)+2)
	fields = append(fields, strconv.Itoa(opcode), strconv.FormatInt(int64(api.index), 10))
	for i, arg := range args {
		if !utf8.ValidString(arg) {
			return "", fmt.Errorf("BeeNT argument %d contains invalid UTF-8", i+1)
		}
		if strings.ContainsRune(arg, 0) || strings.Contains(arg, BeeNTSeparator) {
			return "", fmt.Errorf("BeeNT argument %d contains a reserved protocol delimiter or NUL", i+1)
		}
		fields = append(fields, arg)
	}
	command := encodeFrameworkText(strings.Join(fields, BeeNTSeparator))
	result, err := api.transport.Call([]byte(command))
	if err != nil {
		return "", fmt.Errorf("BeeNT opcode %d: %w", opcode, err)
	}
	if !utf8.Valid(result) {
		return "", fmt.Errorf("BeeNT opcode %d returned invalid UTF-8", opcode)
	}
	return string(result), nil
}

func encodeFrameworkText(value string) string {
	var out strings.Builder
	encoder := simplifiedchinese.GBK.NewEncoder()
	decoder := simplifiedchinese.GBK.NewDecoder()
	for _, r := range value {
		if r < 128 {
			out.WriteRune(r)
			continue
		}
		part := string(r)
		encoded, err := encoder.Bytes([]byte(part))
		if err == nil {
			decoded, decodeErr := decoder.Bytes(encoded)
			if decodeErr == nil && string(decoded) == part {
				out.WriteString(part)
				continue
			}
		}
		for _, unit := range utf16.Encode([]rune{r}) {
			fmt.Fprintf(&out, `\u%04X`, unit)
		}
	}
	return out.String()
}

// DecodeFrameworkText decodes the framework's UTF-16 \uXXXX escapes in plain
// text. Do not apply this to JSON before json.Unmarshal; it can unescape quotes.
// Malformed escapes and unpaired surrogates are preserved unchanged.
func DecodeFrameworkText(value string) string {
	var out strings.Builder
	for i := 0; i < len(value); {
		unit, ok := unicodeUnit(value, i)
		if !ok {
			out.WriteByte(value[i])
			i++
			continue
		}
		if unit >= 0xd800 && unit <= 0xdbff {
			low, valid := unicodeUnit(value, i+6)
			if valid && low >= 0xdc00 && low <= 0xdfff {
				out.WriteRune(utf16.DecodeRune(rune(unit), rune(low)))
				i += 12
				continue
			}
			out.WriteString(value[i : i+6])
			i += 6
			continue
		}
		if unit >= 0xdc00 && unit <= 0xdfff {
			out.WriteString(value[i : i+6])
			i += 6
			continue
		}
		out.WriteRune(rune(unit))
		i += 6
	}
	return out.String()
}

func unicodeUnit(value string, index int) (uint16, bool) {
	if index+6 > len(value) || value[index] != '\\' || value[index+1] != 'u' {
		return 0, false
	}
	v, err := strconv.ParseUint(value[index+2:index+6], 16, 16)
	return uint16(v), err == nil
}

func boolArgument(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

func (api *BeeNTAPI) callBool(opcode int, args ...string) (bool, error) {
	raw, err := api.Call(opcode, args...)
	if err != nil {
		return false, err
	}
	switch strings.TrimSpace(raw) {
	case "1", "真":
		return true, nil
	case "0", "假":
		return false, nil
	default:
		return false, fmt.Errorf("BeeNT opcode %d returned invalid boolean %q", opcode, raw)
	}
}

// GetAppDataDir returns the Worker's directory, which the native bridge creates
// as <host directory>/plugin_data/<PluginName>. It never uses the DLL path.
func GetAppDataDir() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}
func (api *BeeNTAPI) GetAppDataDir() (string, error) { return GetAppDataDir() }
