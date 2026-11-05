// Command buildmeta turns plugin constants into the C shell's metadata/resource inputs.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"

	"golang.org/x/text/encoding/simplifiedchinese"
)

type pluginMetadata struct {
	Name        string `json:"name"`
	Author      string `json:"author"`
	Version     string `json:"ver"`
	Description string `json:"desc"`
}

func readPluginMetadata(reader io.Reader) (pluginMetadata, error) {
	source, err := io.ReadAll(reader)
	if err != nil {
		return pluginMetadata{}, err
	}
	file, err := parser.ParseFile(token.NewFileSet(), "plugin_main.go", source, 0)
	if err != nil {
		return pluginMetadata{}, err
	}
	values := make(map[string]string)
	for _, declaration := range file.Decls {
		decl, ok := declaration.(*ast.GenDecl)
		if !ok || decl.Tok != token.CONST {
			continue
		}
		for _, item := range decl.Specs {
			spec := item.(*ast.ValueSpec)
			for i, name := range spec.Names {
				if i >= len(spec.Values) {
					continue
				}
				literal, ok := spec.Values[i].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					continue
				}
				value, err := strconv.Unquote(literal.Value)
				if err == nil {
					values[name.Name] = value
				}
			}
		}
	}
	meta := pluginMetadata{values["PluginName"], values["PluginAuthor"], values["PluginVersion"], values["PluginDescription"]}
	if meta.Name == "" || meta.Author == "" || meta.Version == "" {
		return pluginMetadata{}, errors.New("plugin_main.go requires literal PluginName, PluginAuthor and PluginVersion string constants")
	}
	return meta, nil
}

func validateName(name string) error {
	if name == "" || name == "." || name == ".." || strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
		return errors.New("PluginName must be a safe Windows directory name")
	}
	for _, ch := range name {
		if ch < 32 || strings.ContainsRune(`<>:"/\|?*`, ch) {
			return errors.New("PluginName contains an invalid Windows filename character")
		}
	}
	stem := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" ||
		(len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9') {
		return errors.New("PluginName is a reserved Windows filename")
	}
	return nil
}

func metadataJSON(meta pluginMetadata) ([]byte, error) {
	if err := validateName(meta.Name); err != nil {
		return nil, err
	}
	if meta.Author == "" || meta.Version == "" {
		return nil, errors.New("author/version cannot be empty")
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	encoded, err := simplifiedchinese.GBK.NewEncoder().Bytes(data)
	if err != nil {
		return nil, fmt.Errorf("metadata must be representable as GBK: %w", err)
	}
	return encoded, nil
}

func generatePluginHeader(meta pluginMetadata) ([]byte, error) {
	data, err := metadataJSON(meta)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	out.WriteString("#ifndef BEENT_PLUGIN_CONFIG_H\n#define BEENT_PLUGIN_CONFIG_H\n#include <windows.h>\n#define BEENT_WORKER_RESOURCE_ID 101\n")
	out.WriteString("static const WCHAR BEENT_PLUGIN_NAME_W[] = {")
	for _, unit := range append(utf16.Encode([]rune(meta.Name)), 0) {
		fmt.Fprintf(&out, "0x%04x,", unit)
	}
	out.WriteString("};\nstatic const unsigned char BEENT_PLUGIN_METADATA_JSON_GBK[] = {\n")
	for i, b := range append(data, 0) {
		if i%16 == 0 {
			out.WriteByte('\n')
		}
		fmt.Fprintf(&out, "0x%02x,", b)
	}
	out.WriteString("\n};\n#endif\n")
	return out.Bytes(), nil
}

func run(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: go run ./other/buildmeta plugin_main.go output-dir")
	}
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()
	meta, err := readPluginMetadata(f)
	if err != nil {
		return err
	}
	header, err := generatePluginHeader(meta)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(args[1], 0755); err != nil {
		return err
	}
	for name, data := range map[string][]byte{
		"plugin_config.h": header,
		"worker.rc":       []byte("101 RCDATA \"beent_go_worker.exe\"\r\n"),
		"plugin_name.txt": []byte(meta.Name),
	} {
		if err := os.WriteFile(filepath.Join(args[1], name), data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "BeeNT metadata:", err)
		os.Exit(1)
	}
}
