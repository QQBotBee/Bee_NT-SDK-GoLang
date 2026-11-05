package main

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestMetadataHeaderUsesBeeNTFieldsAndGBK(t *testing.T) {
	meta, err := readPluginMetadata(strings.NewReader("package main\nconst (PluginName = `测试插件`; PluginAuthor = `作者`; PluginVersion = `1.0`; PluginDescription = `中文说明`)"))
	if err != nil {
		t.Fatal(err)
	}
	header, err := generatePluginHeader(meta)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(header, []byte("BEENT_PLUGIN_METADATA_JSON_GBK")) {
		t.Fatal("missing BeeNT metadata")
	}
	encoded, err := metadataJSON(meta)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != `{"name":"测试插件","author":"作者","ver":"1.0","desc":"中文说明"}` {
		t.Fatalf("metadata: %s", decoded)
	}
}

func TestMetadataRejectsUnsafeWindowsNames(t *testing.T) {
	for _, name := range []string{"", "..", "a/b", `a\b`, "CON", "nul.txt", "a:", "a.", "a "} {
		t.Run(name, func(t *testing.T) {
			_, err := generatePluginHeader(pluginMetadata{Name: name, Author: "a", Version: "1"})
			if err == nil {
				t.Fatalf("accepted unsafe name %q", name)
			}
		})
	}
}

func TestMetadataRequiresLiteralConstants(t *testing.T) {
	_, err := readPluginMetadata(strings.NewReader("package main\nconst PluginName = `a`"))
	if err == nil {
		t.Fatal("accepted missing author/version")
	}
}
