# 仓库约定

## 目录

这是面向 Bee×NT 框架的 Go 插件模板，只支持 Windows 32 位。插件业务写在 `plugin_main.go`：插件信息常量、生命周期回调和消息回调都在这里。`beent_sdk.go` 放协议常量、数据类型和 IPC 定义，`beent_api.go` 放框架 API 1～97 的封装，`worker.go` 放工作进程的 IPC 循环与事件分发，`settings.go` 放原生设置窗口和宿主进程监控。`other/` 里是 C 壳、导出定义、构建期元数据生成器和工作进程入口。`build/` 与 `temp/` 是构建产物，不要提交。

## 构建与检查

- `go test ./...`：跑全部单元测试，是本地改动的最低检查。
- `go vet ./...`：静态检查。
- `build.bat`：用 Go 和 Zig 构建 32 位插件 DLL。
- `build.bat MyPlugin.dll`：指定输出文件名。

构建要求 Windows CMD、PATH 里第一个 `go.exe` 必须是 Go 1.20.14、Zig 在 PATH 里，产物是 `build\插件名.dll`。测试和 vet 也用 Go 1.20.14。本机这个版本在 `D:\Softs\go1.20.14\go\bin`。不要升级发布编译器：兼容 Windows Server 2012 R2 需要钉死这个工具链。`go.mod` 只声明语言版本，`build.bat` 负责核对编译器版本，并在嵌入前核对工作进程的实际版本。

## 编码与命名

用标准 Go 格式，改过的 `.go` 文件跑 `gofmt`。业务逻辑留在 `plugin_main.go`，除非大到值得单独拆文件。回调名与框架概念对应，例如 `onGroupMessage`、`onSettings`。保持 Windows/386 的前提，不要把 Go 运行时放进框架进程。

## 测试

测试文件放在被测代码旁边，命名 `*_test.go`。解析、消息处理和 SDK 行为优先用表驱动测试。提交前 `go test ./...` 必须通过。完整的生命周期、消息回调、设置窗口、工作进程退出和 DLL 卸载稳定性，仍然要在真实的 Bee×NT 环境里验证。

## 提交

提交说明用简短祈使句，例如 `feat: 增加群消息回复` 或 `fix: 保持导出定义的 GBK 编码`。

## 安全与配置

`other/BeeNTPlugin.def` 必须保持 GBK 编码，`.gitattributes` 把它标成 binary，防止换行符或编码被改坏。插件数据通过 `GetAppDataDir()` 写到框架目录的 `plugin_data\插件名\` 下，不要写在 DLL 旁边，也不要写进 `temp_plugin`。
