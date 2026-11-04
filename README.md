<div align="center">
  <h1>Trylet</h1>
  <p>在 Windows 上编写、运行单文件代码并保存到 GitHub</p>
</div>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27.3-00ADD8?style=flat-square&amp;logo=go&amp;logoColor=white" alt="Go 1.27.3" />
  <img src="https://img.shields.io/badge/Wails-3-DF0000?style=flat-square&amp;logo=wails&amp;logoColor=white" alt="Wails 3" />
  <img src="https://img.shields.io/badge/React-19-149ECA?style=flat-square&amp;logo=react&amp;logoColor=white" alt="React 19" />
  <img src="https://img.shields.io/badge/TypeScript-5.9-3178C6?style=flat-square&amp;logo=typescript&amp;logoColor=white" alt="TypeScript 5.9" />
</p>

<p align="center">
  <a href="https://github.com/sunnyhmz7010/Trylet/releases"><img src="https://img.shields.io/github/v/release/sunnyhmz7010/Trylet?label=Release&amp;color=3b82f6" alt="Release" /></a>
  <a href="https://github.com/sunnyhmz7010/Trylet/blob/main/LICENSE"><img src="https://img.shields.io/github/license/sunnyhmz7010/Trylet?color=10b981" alt="License" /></a>
</p>

---

## ✨ 为什么做这个应用

做一道编程题或验证一个想法，往往只需要一个文件。Trylet 将 C、C++、Python 的编写、本机运行和 GitHub 归档放进同一个 Windows 桌面窗口，让你写完就能试、试完就能保存。

- 跟随课程练习：写下完整程序，在内置终端试几组输入，再归档到自己的 GitHub 仓库。
- 整理日常开发内容：找回近期草稿、查看本地 Git 改动，或管理不同工具使用的 AI Skills。

## 🚀 核心能力

- 单文件编辑：支持 C、C++、Python，自动识别语言、插入基础模板、保存草稿和导出源码。
- 交互运行：调用本机解释器或编译器，在内置终端使用 `input()`、`scanf`、`cin` 输入。
- GitHub 归档：选择仓库、分支、目录和文件名，覆盖已有文件前先查看差异。
- 本地 Git 查看：只读浏览文件改动、提交历史、差异和行归属。
- AI Skills 管理：按用户级或项目级安装技能，支持从 GitHub 勾选提取、整仓库安装、更新和删除。
- 桌面快捷入口：通过托盘、全局快捷键和可选悬浮窗打开工作区；当前源码还支持选中文字后显示字符数。
- 便携保存：草稿和设置随 EXE 存放，数据变更时保留一份不含访问令牌的备份。

## ⚡ 快速开始

### 📋 前置要求

- Windows 10 / 11，x64 或 ARM64，已安装 Microsoft Edge WebView2 Runtime。
- 一个可写的独立文件夹，用于存放 EXE、草稿数据和运行缓存。
- 运行 Python 需要本机已有 Python 3；运行 C/C++ 需要 GCC/G++、Clang 或 MSVC。应用不附带运行环境，也不自动安装代码依赖。
- 查看本地 Git 仓库、更新以 Git 克隆方式安装的 Skills 时，需要本机已有 Git。
- 发布代码需要 GitHub 账号、目标仓库和访问令牌；下载公开 Skills 仓库无需登录。

### 📦 安装与运行

1. 打开 [最新发布页](https://github.com/sunnyhmz7010/Trylet/releases/latest)，在 **Assets** 中下载对应架构的 EXE。当前 [v0.1.0](https://github.com/sunnyhmz7010/Trylet/releases/tag/v0.1.0) 提供：

   | 电脑架构 | 下载文件 |
   | --- | --- |
   | x64，Intel / AMD 64 位电脑 | [Trylet-v0.1.0-windows-amd64.exe](https://github.com/sunnyhmz7010/Trylet/releases/download/v0.1.0/Trylet-v0.1.0-windows-amd64.exe) |
   | ARM64，Windows ARM 电脑 | [Trylet-v0.1.0-windows-arm64.exe](https://github.com/sunnyhmz7010/Trylet/releases/download/v0.1.0/Trylet-v0.1.0-windows-arm64.exe) |

2. 将 EXE 放入上述文件夹并双击运行，无需安装 Go 或 Node.js。v0.1.0 启动后先显示悬浮球，点击它或按 `Ctrl+Alt+Space` 打开编辑器。
3. 选择 Python，将编辑区内容替换为下面的代码，按 `Ctrl+Enter`。终端应显示 `Hello, Trylet!`。若提示未找到环境，在“设置 → 运行环境”中选择 Python 解释器或填写完整路径，点击“保存设置”后重试。

   ```python
   print("Hello, Trylet!")
   ```

> 版本差异：后续说明以当前源码为准。当前源码启动时直接打开主窗口，悬浮窗可统计选中文字；v0.1.0 的字符统计需要先复制文字。体验源码中的更新，请参照“本地开发”构建。

## 📖 使用说明

顶部四个图标标签依次是“编辑器”“Git”“Skills”和“设置”，悬停可查看名称。切换页面会保留草稿、终端和浏览状态。

### ✍️ 编写与运行

1. 选择 C、C++ 或 Python，也可使用自动识别；识别不明确时，运行或发布前会提示选择。
2. 编写完整的单文件程序，按 `Ctrl+Enter` 运行，在终端中输入程序需要的内容。再次按下可停止任务。
3. 点击草稿标题可新建、重命名或打开近期练习；“导出当前草稿…”可另存为 `.c`、`.cpp` 或 `.py` 文件。

试试需要输入的 Python 程序：

```python
name = input("你的名字：")
print("你好，" + name + "！")
```

使用 Python 虚拟环境中的包时，在“设置 → 运行环境”选择该环境的 `python.exe`，保存设置后再运行。终端支持右键或 `Ctrl+V` 粘贴。

### ☁️ 发布到 GitHub

1. 打开“设置 → GitHub 发布”，填写默认仓库 `owner/repo`：将 `owner` 替换为用户名或组织名，`repo` 替换为仓库名。默认分支可留空，使用仓库默认分支。
2. 点击“创建细粒度令牌”，在 GitHub 页面手动选择目标仓库，确认 `Contents: Read and write` 权限并生成令牌。
3. 将令牌粘贴回设置，点击“保存设置”。返回编辑器点击“发布”，选择分支和目录，检查文件名、最终路径及提交说明后确认。
4. 目标文件已有不同内容时，先查看差异，再选择更新原文件、另存新文件或取消；内容一致时不会重复提交。

### 🌿 查看本地 Git 仓库

打开“Git”，点击左上角文件夹入口选择本地仓库。左侧区分已暂存、未暂存、未跟踪和冲突文件；右侧查看文件差异或提交历史，支持路径筛选、差异搜索和行归属。

在“设置 → Git 浏览”选择左右对照或统一差异；面板过窄时自动改为统一差异。Git 页面只读，暂存、提交和同步需使用其他 Git 工具；领先、落后状态依据本地已有远端引用显示，不自动联网刷新。

### 🧩 管理 AI Skills

1. 选择“用户级”或“项目级”，再选择共享 `.agents/skills`、Codex 的 `.codex/skills`、Claude Code 的 `.claude/skills` 或自定义目录。项目级需先选择项目文件夹。
2. 点击“下载”，输入 GitHub HTTPS 仓库地址或 `owner/repo`，其中 `owner`、`repo` 分别替换为实际所有者和仓库名。
3. 保留“自动提取 skills”，点击“下载并识别”，勾选需要的技能后安装。若技能依赖仓库中的共享文件，取消“自动提取 skills”，改为整仓库安装。
4. 在列表中搜索、展开详情、更新或删除技能；删除前核对弹窗显示的路径和范围。

通过 Trylet 安装的技能会自动记录来源；已有 Git 克隆可从 GitHub `origin` 识别来源。未绑定来源的技能无法直接更新。下载私有仓库复用设置中的令牌，需要该仓库的 `Contents` 读取权限。

下载会显示连接、下载、解压和识别的实际阶段，可取消下载或关闭弹窗；取消后清理预览。安装写入期间需等待完成。

### 🫧 悬浮窗与选区统计

在“设置 → 常规”管理悬浮窗。默认与主窗口联动：主窗口显示时隐藏，最小化或后台运行时显示；关闭联动可让两者同时显示。

在其他应用中选中文字，支持 Windows UI Automation 文字选区的控件会让悬浮窗展开，显示计空格、不计空格的字符数；取消选中后收起。文本仅在本机临时读取，不保存；无法提供文字选区的控件保持收起。

首次关闭主窗口时，可选择“退出应用”或“最小化到托盘”，也可记住选择。之后可在“设置 → 常规”修改。

### ⌨️ 常用快捷键

| 操作 | 默认快捷键 |
| --- | --- |
| 全局打开、收起主窗口 | `Ctrl+Alt+Space`，可在设置中修改或关闭 |
| 运行、停止 | `Ctrl+Enter` |
| 新建练习 | `Ctrl+N` |
| 保存当前草稿 | `Ctrl+S`，平时自动保存 |
| 发布到 GitHub | `Ctrl+Shift+S` |

## 🧠 功能细节

### 💾 数据保存与恢复

| 位置 | 内容与用途 |
| --- | --- |
| EXE 同级 `Trylet.json` | 自动生成，保存草稿、设置、发布记录、Git 偏好、Skills 来源和加密令牌 |
| EXE 同级 `Trylet.backup.json` | 上一份数据的备份，随保存更新，不含访问令牌 |
| EXE 同级 `.Trylet-runtime` | 缓存、运行产物和 Skills 下载预览，正常退出后清理；异常遗留在下次启动时重试清理 |
| 所选 Skills 安装目录 | 技能文件本体，需要单独备份 |

移动应用时将 `Trylet.json` 与 EXE 一起保留。令牌使用 Windows DPAPI 加密，更换电脑或 Windows 账户后需重新填写。主文件损坏且备份有效时，启动会询问是否恢复，并先另存损坏文件；备份可能不含最后一次编辑，恢复后需重新填写令牌。

### ⚙️ 代码如何运行

- 使用点击运行时的源码快照，同时只运行一个任务，停止时结束任务及子进程。
- C/C++ 默认采用 C17/C++17，编译最长 60 秒；交互程序等待输入不设默认超时。
- 代码以当前 Windows 用户权限执行，不提供文件系统沙箱。程序生成的相对路径文件位于运行临时目录，需要保留时请在退出应用前复制。

### 🔄 更新与覆盖规则

- GitHub 发布通过 REST API 写入文件；确认后远端再次变化时，会重新展示差异。网络结果不确定时先读取远端核对。
- Skills 下载使用仓库默认分支的 ZIP 快照，不执行仓库脚本。同名目录不自动覆盖，更新前会要求确认本地改动；提取安装仅更新原先选中的技能。
- “Git 克隆”技能使用本机 Git 和已有凭据快进更新；工作区有改动、分支分叉或未跟踪 `origin` 上游时停止并提示处理。
- `.system` 内置技能只读；删除目录链接时保留链接指向的原始目录。

## 🧱 技术栈

- Go、Wails 3：桌面应用、托盘、全局快捷键及前后端接口。
- React 19、TypeScript、Vite：工作区界面与前端构建。
- Tailwind CSS 4、shadcn/ui、Radix UI：样式与交互组件。
- CodeMirror 6：代码编辑、语法高亮和差异展示。
- xterm.js、go-pty、ConPTY：终端显示与 Windows 交互进程。
- WebView2、Windows UI Automation：界面渲染与文字选区读取。
- Windows DPAPI：访问令牌的本机加密保存。
- Git、GitHub REST API：本地仓库读取、Skills 更新和远端文件归档。

## 🗂️ 项目结构

```text
Trylet/
├── cmd/trylet/             # Windows 应用入口
├── internal/
│   ├── app/               # 草稿、设置与任务协调
│   ├── desktop/           # 窗口、托盘、快捷键与选区读取
│   ├── git/               # 本地仓库浏览与 Skills Git 更新
│   ├── github/            # GitHub API 与仓库下载
│   ├── publish/           # 发布请求与结果类型
│   ├── runner/            # 环境检测、编译与终端运行
│   ├── skills/            # 技能扫描、安装、更新与删除
│   ├── store/             # 数据保存、备份与令牌加密
│   ├── textstats/         # 选中文字的字符统计
│   └── portable/          # 同级运行目录与退出清理
├── frontend/
│   ├── src/app/           # 主窗口、导航与页面协调
│   ├── src/features/      # 编辑器、Git、Skills、设置与发布
│   ├── src/components/ui/ # 共用界面组件
│   └── bindings/          # Wails 生成的接口与类型
├── build/                 # 图标与 Windows 资源
└── build.ps1              # Windows 构建入口
```

## 👨‍💻 本地开发

### 🧰 环境

- 开发平台：Windows 10 / 11，x64 或 ARM64。
- Go：按 `go.mod` 使用 1.27.3 工具链。
- Node.js：22.12 或更新版本，使用 npm。
- Git、PowerShell：用于获取源码和执行构建脚本。
- Microsoft Edge WebView2 Runtime：运行桌面应用。

### 🔨 构建并启动

```powershell
git clone https://github.com/sunnyhmz7010/Trylet.git
cd Trylet

# 构建并启动，产物为 bin/Trylet.exe
.\build.ps1 -Run
```

### 🧪 本地检查

```powershell
# 首次构建完成后，在仓库根目录执行
go test -count=1 ./...
go vet ./...
npm --prefix frontend test
npm --prefix frontend run check
```

## 🔐 安全报告

如果发现安全问题，请不要公开披露细节。请优先参考仓库中的 [SECURITY.md](./SECURITY.md) 提交安全报告。

## 📄 许可证

本项目基于 [GPL-3.0](./LICENSE) 开源。

<div align="center">
  <sub>Built with ❤️ by Sunny</sub>
</div>
