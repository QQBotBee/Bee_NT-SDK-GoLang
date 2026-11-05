# BeeNT Go SDK 设计

## 目标和约束

基于相邻 `BeeSDK-GoLang` 的单 DLL 架构，实现能被 BeeNT 加载的独立 Go SDK 开发模板。依据 `BeeNT框架参考` 中初始化模板、插件信息类、常量表、SDK、API 接收处理和框架内部实现核对协议。Windows/386，统一固定 Go 1.20.14，CGO_ENABLED=0，Go Runtime 只进入独立 Worker。宿主文本边界 CP936/GBK，Go 和 IPC 内容 UTF-8。

## 实现方案

保留原生 C 壳、资源内嵌 Worker 和原生设置窗口。直接将 Go Runtime 放入 DLL 会重新引入热卸载风险；只替换原 Bee 导出名则无法满足 BeeNT 的初始化和 API 协议。因此采用独立 BeeNT 模板，复用成熟的构建和窗口机制，重写框架适配层。

`botPluginLoad(core,index)` 只保存核心入口与索引，并返回 `name/author/ver/desc` 以及 `enable/disable/uninstall/setting/private/group/event` 七个 C 回调地址。初始化不启动 Worker；第一次启用或打开设置时启动。生命周期无参数，好友消息三个文本参数，群消息四个，事件八个，消息处理结果 0/1。

API 核心入口为 stdcall 单文本输入、文本指针输出（用户补充的 `易语言_插件API接收处理.txt`）；命令为操作码、插件索引和业务参数，以 `#|&BEExNT&|#` 连接。返回指针先复制，不擅自释放框架拥有的内存。SDK 提供 1–97 具名操作、类型化信息和列表、31 个事件值以及好友/群便捷入口。按接收端和内部实现修正原模板 45/96/97 的字段、参数错误，94 保持真实文本结果语义。

## IPC 与生命周期

JSON Lines 保留 `event/api_call/api_result/event_result` 消息和 Base64 文本载荷。Worker 唯一读取循环按请求 ID 分发 API 结果，业务事件顺序处理；并发 API 请求独立等待并具备超时。C 壳持续服务后台 API 并严格关联事件结果。原生泵线程持有模块引用，防止宿主卸载仍在执行的原生代码。禁用必须停止 Worker，因为 BeeNT 重载只调用禁用后直接 FreeLibrary。宿主退出通过父进程监控和管道 EOF 结束 Worker。

数据目录从宿主 EXE 所在目录确定：`plugin_data/<PluginName>`。日志写 stderr，stdout 专供协议；设置窗口沿用原生模板。插件默认示例避免自动给真实用户发消息。

## 验证和范围

单元测试覆盖完整操作码、参数顺序、布尔值、列表短记录、64 位时间戳、分隔符、GBK 与 Unicode 边界；IPC 测试覆盖乱序并发结果、EOF、超时、panic、生命周期。构建 Windows/386 Worker 和 PE32 DLL，并使用原生模拟宿主实际 LoadLibrary 检查回调地址、GBK、调用返回、禁用和重复加载。真实 BeeNT 账号/网络、宿主线程约束、框架文本返回所有权仍需在真实宿主验收。
