<div align="center">
  <img src="build/icon.svg" alt="Trylet Logo" width="88" />
  <h1>Trylet</h1>
  <p>在 Windows 上随手编写、运行单文件代码，并归档到 GitHub。</p>
</div>

<p align="center">
  <a href="https://github.com/sunnyhmz7010/Trylet"><img src="https://img.shields.io/badge/Windows-10%20%2F%2011-3b82f6" alt="Windows 10 / 11" /></a>
  <a href="https://github.com/sunnyhmz7010/Trylet/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-GPL--3.0-10b981" alt="GPL-3.0" /></a>
</p>

---

## ✨ 为什么做这个应用

跟着课程练习时，打开一个小窗口就能写代码、测试输入输出，再把练习提交到自己的仓库。Trylet 用悬浮球和全局快捷键缩短这个过程，也在同一窗口提供本地 Git 仓库的只读浏览。

## 🚀 核心能力

- 悬浮球、托盘和可配置快捷键，按需展开编辑窗口。
- C、C++、Python 单文件编辑，自动识别语言、模板、草稿自动保存与近期历史。
- 检测本机 Python、GCC/G++、Clang、MSVC，支持手动指定环境和交互终端。
- 发布到指定 GitHub 仓库，选择目录、比较远端差异、确认后更新文件。
- 本地 Git 更改、提交图、文件差异和行归属浏览，整合 Oil Git 界面。
- 无安装器；应用数据集中存入 EXE 同级的一个 JSON 文件。

## ⚡ 快速开始

### 📋 前置要求

- Windows 10/11 x64，系统已有 Microsoft Edge WebView2 Runtime。
- 运行代码需要本机已有 Python 3 或 C/C++ 编译器；应用不附带这些环境。
- 浏览本地仓库需要 Git。GitHub 发布使用 REST API，无需本地克隆。
- EXE 所在目录必须可写。

### 📦 安装与运行

已有便携包时，解压后直接打开 `Trylet.exe`。从源码构建需要 Go 和 Node.js：

```powershell
git clone https://github.com/sunnyhmz7010/Trylet.git
cd Trylet
.\build.ps1 -Run
```

构建产物为 `bin\Trylet.exe`。当前仓库尚未发布 GitHub Release。

## 📖 使用说明

点击悬浮球或按 `Ctrl+Alt+Space` 打开编辑器。顶部三个图标标签依次进入编辑器、Git 和设置。选择语言后编写代码，按 `Ctrl+Enter` 运行；交互终端支持 `scanf`、`cin`、`input()`。

```python
name = input("你的名字：")
print(f"你好，{name}！")
```

在设置中填写 `owner/repo` 和细粒度令牌，再从编辑器点击“发布”。令牌创建链接只预填仓库所属账户与 Contents 读写权限，目标仓库由用户在 GitHub 页面选择。文件已存在且内容不同时，先展示差异再确认更新。

Git 标签中选择本地仓库后可查看更改和历史；该页面不提供提交、切换分支或推送操作。完整操作和快捷键见 [使用说明](./使用说明.md)。

## 🧠 功能细节

长期数据仅保存在 EXE 同级的 `Trylet.json`，包含草稿、设置、发布记录和 Git 偏好。令牌经当前 Windows 账户加密后存入同一文件，更换账户或电脑后需重新填写。备份时将 EXE 与数据文件一起带走。

运行时创建同级 `.Trylet-runtime`，集中存放 WebView2 缓存、编译产物和临时文件；正常退出后清理。目录不可写时明确报错。异常退出留下的临时目录会在下次启动时重试清理。用户代码以当前用户权限执行，未做文件系统沙箱。

运行使用点击时的源码快照，同时只运行一个任务；输出与会话关联，停止时结束进程树。C/C++ 默认使用 C17/C++17，交互输入不设默认等待超时。

发布更新携带确认过的文件 SHA；并发修改会重新展示差异，请求结果不确定时先核对远端，避免重复提交。

## 🧱 技术栈

- 桌面与后端：Wails 3、Go、系统 WebView2、ConPTY。
- 前端：React、TypeScript、Vite、Tailwind CSS、按需 shadcn/ui。
- 编辑与终端：CodeMirror 6、xterm.js。
- 存储与发布：本地 JSON、Windows DPAPI、GitHub REST API。
- 目标平台：Windows 10/11 x64。

## 🗂️ 项目结构

```text
Trylet/
├── main.go                  # 桌面窗口、托盘和快捷键
├── portable.go              # 同级运行目录与退出清理
├── internal/trylet/         # 草稿、运行环境、执行、Git 与发布
├── frontend/src/            # 编辑器、Git、设置和悬浮球
├── frontend/bindings/       # Wails 生成的类型与接口
├── build/                  # 图标和 Windows 资源
├── build.ps1               # Windows 构建入口
└── THIRD_PARTY_NOTICES.txt  # 第三方版权和许可证
```

## 👨‍💻 本地开发

### 🧰 环境

Go 工具链固定为 1.27.3，Node.js 22.12 或更新版本。首次构建需要联网获取已锁定的依赖。运行集成测试使用本机已有的编译器和 Python，缺少的环境会明确跳过。

### ⚙️ 命令

```powershell
.\build.ps1
go test -count=1 ./...
go vet ./...
npm --prefix frontend test
npm --prefix frontend run build
```

先构建以生成 Go 嵌入的前端资源。模拟 GitHub API 测试只监听回环地址，不读取真实令牌或写入真实仓库。

## 🔐 安全报告

如果发现安全问题，请不要公开披露细节。请优先参考仓库中的 [SECURITY.md](./SECURITY.md) 提交安全报告。

## 📄 许可证

本项目基于 [GPL-3.0](./LICENSE) 开源。第三方组件保留各自许可证，Oil Git 等组件的来源与版权见 [THIRD_PARTY_NOTICES.txt](./THIRD_PARTY_NOTICES.txt)。

<div align="center">
  <sub>Built with ❤️ by Sunny</sub>
</div>
