# BeeNT Go SDK 文档导航

普通插件开发按以下顺序进行：

1. 阅读 [README 快速开始](../README.md#快速开始)，修改插件信息和业务。
2. 使用 [AI 开发指南](AI开发指南.md) 中的提示词，让 AI 了解实际接口与开发边界。
3. 按 [API 参考](API参考.md) 选择所需方法；需要设置界面时阅读 [设置窗口开发规范](设置窗口开发规范.md)。
4. 运行根目录 `build.bat`，将生成的 DLL 导入真实 BeeNT，验证自己的实际功能。

所有构建统一使用 Go 1.20.14，根目录只有一个构建入口，`build/` 只交付最终 DLL。

## SDK 维护资料

- [AGENTS.md](../AGENTS.md)：代码职责、协议边界和维护约定。
- [other/README.md](../other/README.md)：原生壳、构建内部实现与回归工具。
- [验证记录](验证记录.md)：当前工具链、内部检查和实机反馈的范围。

## 历史记录

- [SDK 1.0.1 实机操作记录](history/BeeNT实机验收.md)
- [早期构建与验证记录](history/2026-11-05-早期验证记录.md)
- [最初 SDK 设计](superpowers/specs/2026-11-05-beent-sdk-design.md)
- [最初 SDK 实施计划](superpowers/plans/2026-11-05-beent-sdk.md)

历史文件保留实现与问题排查依据。当前构建方式、目录和工具链以 README 与源码为准。
