// 构建工具：读取 plugin_main.go 的插件常量，生成 C 壳需要的元数据头和资源脚本。
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
	"golang.org/x/text/transform"
)

const workerResourceID = 101

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
	values := map[string]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for index, name := range spec.Names {
			if index >= len(spec.Values) {
				continue
			}
			literal, ok := spec.Values[index].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				continue
			}
			value, unquoteErr := strconv.Unquote(literal.Value)
			if unquoteErr == nil {
				values[name.Name] = value
			}
		}
		return true
	})
	metadata := pluginMetadata{values["PluginName"], values["PluginAuthor"], values["PluginVersion"], values["PluginDescription"]}
	if metadata.Name == "" || metadata.Author == "" || metadata.Version == "" {
		return pluginMetadata{}, errors.New("plugin_main.go 缺少 PluginName、PluginAuthor 或 PluginVersion 字符串常量")
	}
	if err := validateName(metadata.Name); err != nil {
		return pluginMetadata{}, err
	}
	return metadata, nil
}

func validateName(name string) error {
	if name == "." || name == ".." || strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
		return errors.New("PluginName 不能作为 Windows 目录名")
	}
	for _, ch := range name {
		if ch < 32 || strings.ContainsRune(`<>:"/\|?*`, ch) {
			return errors.New("PluginName 含有 Windows 文件名禁止的字符")
		}
	}
	stem := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	reserved := stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" ||
		(len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '1' && stem[3] <= '9')
	if reserved {
		return errors.New("PluginName 是 Windows 保留名")
	}
	return nil
}

func encodeGBK(value []byte) ([]byte, error) {
	encoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), value)
	return encoded, err
}

func generatePluginHeader(metadata pluginMetadata) ([]byte, error) {
	jsonData, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	gbkJSON, err := encodeGBK(jsonData)
	if err != nil {
		return nil, fmt.Errorf("插件信息无法用 GBK 表示: %w", err)
	}
	gbkJSON = append(gbkJSON, 0)
	units := append(utf16.Encode([]rune(metadata.Name)), 0)
	var output bytes.Buffer
	output.WriteString("#ifndef BEENT_PLUGIN_CONFIG_H\n#define BEENT_PLUGIN_CONFIG_H\n\n#include <windows.h>\n\n")
	fmt.Fprintf(&output, "#define BEENT_WORKER_RESOURCE_ID %d\n#define BEENT_WORKER_FILENAME L\"beent_go_worker.exe\"\n\n", workerResourceID)
	output.WriteString("static const WCHAR BEENT_PLUGIN_NAME_W[] = {")
	for index, unit := range units {
		if index > 0 {
			output.WriteString(",")
		}
		fmt.Fprintf(&output, "0x%04x", unit)
	}
	output.WriteString("};\n\nstatic const unsigned char BEENT_PLUGIN_METADATA_JSON_GBK[] = {\n")
	for index, value := range gbkJSON {
		if index%12 == 0 {
			output.WriteString("    ")
		}
		fmt.Fprintf(&output, "0x%02x,", value)
		if index%12 == 11 || index == len(gbkJSON)-1 {
			output.WriteByte('\n')
		} else {
			output.WriteByte(' ')
		}
	}
	output.WriteString("};\n\n#endif\n")
	return output.Bytes(), nil
}

func materializeWorkerRuntime(projectDir string) error {
	source, err := os.ReadFile(filepath.Join(projectDir, "other", "worker_runtime.go"))
	if err != nil {
		return fmt.Errorf("读取工作进程运行时: %w", err)
	}
	source = bytes.ReplaceAll(source, []byte("\r\n"), []byte("\n"))
	const buildTag = "//go:build beent_worker_runtime\n\n"
	if !bytes.HasPrefix(source, []byte(buildTag)) {
		return errors.New("other/worker_runtime.go 缺少 beent_worker_runtime 构建标签")
	}
	return os.WriteFile(filepath.Join(projectDir, "worker_runtime.go"), bytes.TrimPrefix(source, []byte(buildTag)), 0o644)
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "用法: go run ./other/buildmeta plugin_main.go 输出目录")
		os.Exit(2)
	}
	source, err := os.Open(os.Args[1])
	if err != nil {
		fail(err)
	}
	metadata, err := readPluginMetadata(source)
	source.Close()
	if err != nil {
		fail(err)
	}
	header, err := generatePluginHeader(metadata)
	if err != nil {
		fail(err)
	}
	if err := os.MkdirAll(os.Args[2], 0o755); err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(os.Args[2], "plugin_config.h"), header, 0o644); err != nil {
		fail(err)
	}
	rc := fmt.Sprintf("%d RCDATA \"beent_go_worker.exe\"\r\n", workerResourceID)
	if err := os.WriteFile(filepath.Join(os.Args[2], "worker.rc"), []byte(rc), 0o644); err != nil {
		fail(err)
	}
	if err := materializeWorkerRuntime(filepath.Dir(os.Args[1])); err != nil {
		fail(err)
	}
	gbkName, err := encodeGBK([]byte(metadata.Name))
	if err != nil {
		fail(fmt.Errorf("插件名称无法用 GBK 表示: %w", err))
	}
	if _, err := os.Stdout.Write(append(gbkName, '\r', '\n')); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "元数据生成失败:", err)
	os.Exit(1)
}
