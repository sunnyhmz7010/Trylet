<div align="center">
  <!-- <img src="build/icon.svg" alt="Trylet Logo" width="120" /> -->
  <h1>Trylet</h1>
  <p>在 Windows 上随手写代码、运行练习、归档到 GitHub，也能查看 Git 和管理 AI Skills</p>
</div>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.27.3-00ADD8?style=flat-square&amp;logo=go&amp;logoColor=white" alt="Go 1.27.3" />
</p>

<p align="center">
  <!-- 无 Release 时可改为 Tag 徽章：https://img.shields.io/github/v/tag/sunnyhmz7010/Trylet?label=Tag&color=3b82f6 -->
  <a href="https://github.com/sunnyhmz7010/Trylet/releases"><img src="https://img.shields.io/github/v/release/sunnyhmz7010/Trylet?label=Release&amp;color=3b82f6" alt="Release" /></a>
  <a href="https://github.com/sunnyhmz7010/Trylet/blob/main/LICENSE"><img src="https://img.shields.io/github/license/sunnyhmz7010/Trylet?color=10b981" alt="License" /></a>
  <!-- 若无 CI，请直接删除下一行的 CI 徽章 -->
  <a href="https://github.com/sunnyhmz7010/Trylet/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/sunnyhmz7010/Trylet/ci.yml?branch=main&amp;label=CI" alt="CI" /></a>
</p>

---

## ✨ 为什么做这个应用

跟着课程练习 C、C++ 或 Python 时，常常只想写一个文件、试几组输入，再把练习留存下来。Trylet 把编辑、交互运行和 GitHub 归档放进一个 Windows 桌面窗口，并提供本地 Git 浏览与 AI Skills 管理，方便在练习和日常开发之间切换。

- 学习编程：编写单文件练习，使用本机环境运行，通过自动保存和近期历史继续上次的进度。
- 日常开发：查看仓库改动与提交历史，集中安装和更新用户级、项目级 AI Skills。

## 🚀 核心能力

- 单文件编辑：编写 C、C++、Python，自动识别语言、插入模板、保存草稿，并导出源码文件。
- 交互运行：检测本机 Python、GCC/G++、Clang、MSVC，在内置终端中使用 `input()`、`scanf`、`cin`。
- GitHub 归档：选择仓库、分支和目录发布代码，文件重名时先比较差异再决定是否更新。
- 本地 Git 浏览：查看暂存、未暂存、未跟踪与冲突文件，以及提交历史、文件差异和行归属。
- AI Skills 管理：浏览已有技能，从 GitHub 按需提取或整仓库安装，绑定来源后更新、删除。
- 桌面快捷入口：通过悬浮窗、托盘和全局快捷键打开主窗口，按需调整窗口大小与置顶状态。
- 选区字符统计：在支持读取文字选区的应用中，选中文字即可在悬浮窗查看计空格、不计空格的字符数。
- 便携保存：应用数据存放在 EXE 同级目录，自动保留一份不含访问令牌的数据备份。

## ⚡ 快速开始

### 📋 前置要求

- Windows 10 / 11，x64 或 ARM64 架构，已安装对应架构的 Microsoft Edge WebView2 Runtime。
- EXE 所在目录可写，用于保存草稿、设置和运行缓存。
- 运行 Python 代码需要已有 Python 3；运行 C/C++ 需要已有编译器。Trylet 使用你安装的环境和依赖。
- 查看本地 Git 仓库、更新通过 Git 克隆的 Skills 时，需要本机已安装 Git。
- 发布到 GitHub 时，需要 GitHub 账号、有写入权限的仓库及访问令牌；公开 Skills 仓库可匿名下载。

### 📦 安装与运行

1. 打开 [最新发布页](https://github.com/sunnyhmz7010/Trylet/releases/latest)，在 **Assets** 中按电脑架构下载 EXE。已发布的 v0.1.0 提供以下文件：

   | 电脑架构 | 下载文件 |
   | --- | --- |
   | x64，常见的 Intel / AMD 64 位电脑 | [Trylet-v0.1.0-windows-amd64.exe](https://github.com/sunnyhmz7010/Trylet/releases/download/v0.1.0/Trylet-v0.1.0-windows-amd64.exe) |
   | ARM64，Windows ARM 电脑 | [Trylet-v0.1.0-windows-arm64.exe](https://github.com/sunnyhmz7010/Trylet/releases/download/v0.1.0/Trylet-v0.1.0-windows-arm64.exe) |

2. 将 EXE 放入可写的独立文件夹，双击运行。v0.1.0 启动后先显示悬浮球，点击它或按 `Ctrl+Alt+Space` 打开编辑器。
3. 选择 Python，输入 `print("Hello, Trylet!")`，按 `Ctrl+Enter`。检测到可用 Python 后，终端应显示 `Hello, Trylet!`；未检测到环境时，按提示进入“设置 → 运行环境”选择或填写解释器路径。

本文后续功能与界面说明以当前源码为准。当前源码启动时直接打开主窗口，并支持无需复制的选区字符统计；体验这些后续更新，请按“本地开发”一节构建。

## 📖 使用说明

顶部四个图标标签依次是“编辑器”“Git”“Skills”和“设置”，悬停可查看名称。详细操作可查阅 [使用说明](./使用说明.md)。

### 编写与运行

1. 在编辑器中选择 C、C++ 或 Python，也可保留自动识别。识别不明确时，运行或发布前会要求选择语言。
2. 编写完整的单文件程序，按 `Ctrl+Enter` 运行，在终端中输入程序需要的内容。
3. 点击草稿标题可新建、重命名或打开近期练习；草稿菜单中的“导出当前草稿…”可另存源码。

下面的 Python 示例可用于体验交互输入：

```python
name = input("你的名字：")
print("你好，" + name + "！")
```

“设置 → 运行环境”可以重新检测环境、切换候选项，或填写 `python.exe`、编译器的完整路径。运行时使用所选环境中已有的依赖。

### 发布到 GitHub

1. 打开“设置 → GitHub 发布”，填写 `owner/repo`。默认分支留空时使用仓库的默认分支。
2. 点击“创建细粒度令牌”。创建页会预填仓库所属账户和 `Contents: Read and write` 权限，仍需手动选择目标仓库并生成令牌。
3. 将令牌粘贴回设置，点击“保存设置”。
4. 返回编辑器，点击“发布”，选择分支、目录，核对文件名和提交说明后确认。
5. 若目标文件已有不同内容，先比较差异，再选择更新原文件、另存为新文件或取消。内容相同时不会重复提交。

### 查看本地 Git 仓库

打开“Git”标签，通过左上角文件夹入口选择本地仓库。左侧查看文件改动，右侧查看差异或提交历史；可按路径筛选、搜索差异、查看行归属。在“设置 → Git 浏览”中统一选择左右对照或统一差异，窗口宽度不足时自动使用统一差异。

Git 页面只读取本地仓库，不提供暂存、提交、切换分支、拉取或推送操作。远端领先、落后状态依据本地已有的远端引用显示。

### 管理 AI Skills

1. 打开“Skills”标签，选择用户级或项目级，再选择共享 `.agents/skills`、Codex 的 `.codex/skills`、Claude Code 的 `.claude/skills`，或自定义目录。
2. 点击“下载”，输入 GitHub HTTPS 仓库地址或 `owner/repo`。
3. 保留“自动提取 skills”以按技能安装；技能依赖仓库内其他共享文件时，可取消勾选，保留完整仓库目录。
4. 点击“下载并识别”，勾选所需技能，确认目标位置后安装。已绑定来源的技能可在列表中更新，删除前会展示完整路径和范围。

已有 Git 克隆目录会读取 GitHub `origin` 和当前提交，标记为“Git 克隆”；没有安装记录或 Git 来源的技能显示为未绑定来源，无法直接更新。公开仓库无需令牌，私有仓库使用设置中具有目标仓库读取权限的令牌。

### 悬浮窗与字符统计

在“设置 → 常规”开启悬浮窗。默认联动主窗口：主窗口显示时隐藏悬浮窗，最小化或后台运行时显示；取消“联动主窗口”可让两者同时显示。

在支持 Windows UI Automation 文字选区的控件中选中文字，悬浮窗会自动展开，显示“字符数（不计空格）”与“字符数（计空格）”；取消选中后恢复圆形入口。无需复制，选区文本只在本机临时读取，不保存。无法提供文字选区的控件不会显示统计。

点击悬浮窗或托盘图标可打开主窗口。首次关闭主窗口时，会询问退出还是最小化到托盘，可勾选记住选择；之后也可在“设置 → 常规”修改。

### 常用快捷键

| 操作 | 默认快捷键 |
| --- | --- |
| 全局打开、收起主窗口 | `Ctrl+Alt+Space`，可在设置中修改或关闭 |
| 运行、停止 | `Ctrl+Enter` |
| 新建练习 | `Ctrl+N` |
| 保存当前草稿 | `Ctrl+S`，平时自动保存 |
| 发布到 GitHub | `Ctrl+Shift+S` |

## 🧠 功能细节

### 数据与备份

| 位置 | 内容与用途 |
| --- | --- |
| EXE 同级 `Trylet.json` | 草稿、设置、发布记录、Git 偏好、Skills 来源绑定及加密令牌 |
| EXE 同级 `Trylet.backup.json` | 上一次完整数据的备份，只保留一份，不包含访问令牌 |
| EXE 同级 `.Trylet-runtime` | WebView2 缓存、编译产物、临时文件和 Skills 下载预览，正常退出后清理 |
| 所选 Skills 安装目录 | 技能文件本体，需单独备份 |

备份或移动应用时保留 `Trylet.json`。GitHub 令牌由当前 Windows 账户加密，更换电脑或账户后需重新填写。主文件损坏且备份有效时，启动会询问是否恢复，恢复前先另存损坏文件；备份可能不含最后一次编辑。

### 运行方式

每次运行使用点击时的源码快照，同时只运行一个任务，停止时结束进程树。C/C++ 默认采用 C17/C++17，编译最长等待 60 秒，交互程序等待输入不设默认超时。用户代码以当前 Windows 用户权限运行，不提供文件系统沙箱。

### 更新与覆盖规则

- GitHub 发布通过 REST API 写入文件，携带确认过的文件 SHA。远端再次变动时重新展示差异；请求结果不确定时先核对远端，保留本地草稿与表单。
- Trylet 下载的 Skills 使用仓库默认分支的 ZIP 快照，安装过程不执行仓库脚本。同名目录不自动覆盖；更新发现本地改动时先确认，提取安装仅更新原来选择的技能。
- “Git 克隆”技能通过本机 Git 和已有凭据快进更新。工作区有改动、分支分叉或缺少 `origin` 上游时停止，交由用户处理。
- 内置 `.system` 技能只读，删除目录链接时保留链接指向的原始目录。

## 🧱 技术栈

- Go 与 Wails 3：桌面应用、窗口管理和前后端接口。
- React、TypeScript、Vite：工作区页面与前端构建。
- Tailwind CSS、shadcn/ui：界面样式和交互组件。
- CodeMirror 6：代码编辑、语言语法与差异展示。
- xterm.js、ConPTY：终端显示和 Windows 交互进程。
- 系统 WebView2、Windows UI Automation：界面渲染与文字选区读取。
- 本地 JSON、Windows DPAPI、GitHub REST API：数据保存、令牌加密和远端发布。

Oil Git 等第三方组件的来源、版权和许可证见 [THIRD_PARTY_NOTICES.txt](./THIRD_PARTY_NOTICES.txt)。

## 🗂️ 项目结构

```text
Trylet/
├── cmd/trylet/             # Windows 应用入口
├── internal/
│   ├── app/               # 草稿、设置、运行与发布协调
│   ├── desktop/           # 窗口、托盘、快捷键与选区读取
│   ├── git/               # 本地仓库浏览与 Skills Git 更新
│   ├── github/            # GitHub 文件发布与仓库下载
│   ├── runner/            # 环境检测、编译与终端运行
│   ├── skills/            # 技能扫描、安装、更新与删除
│   ├── store/             # 数据保存、备份与令牌加密
│   ├── portable/          # 同级运行目录与退出清理
│   └── textstats/         # 选中文字的字符统计
├── frontend/
│   ├── src/app/           # 主窗口、导航与页面协调
│   ├── src/features/      # 编辑器、Git、Skills、设置与发布
│   ├── src/components/ui/ # 共用界面组件
│   ├── bindings/          # Wails 生成的接口与类型
│   └── scripts/           # 构建产物体积检查
├── build/                 # 图标与 Windows 资源
├── build.ps1              # Windows 构建入口
├── 使用说明.md            # 完整操作说明
└── THIRD_PARTY_NOTICES.txt # 第三方版权与许可证
```

## 👨‍💻 本地开发

### 🧰 环境

- 开发平台：Windows 10 / 11，x64 或 ARM64。
- Go：`go.mod` 声明 Go 1.27.0，工具链固定为 1.27.3。
- Node.js：建议 22.12 或更新版本，使用 npm 和仓库内的锁文件。
- Git、PowerShell：用于获取源码和执行构建脚本。
- Microsoft Edge WebView2 Runtime：运行桌面应用。

首次构建需要联网获取依赖。`build.ps1` 会安装锁定的前端依赖，生成 Wails 绑定、前端资源和 Windows 资源，再构建当前目标架构的 EXE。运行环境集成测试使用本机已有的 Python、C/C++ 编译器，缺少的环境会跳过对应测试。

### ⚙️ 命令

```powershell
git clone https://github.com/sunnyhmz7010/Trylet.git
cd Trylet

# 构建并启动，产物为 bin/Trylet.exe
.\build.ps1 -Run

# 在首次构建生成前端资源后执行检查
go test -count=1 ./...
go vet ./...
npm --prefix frontend test
npm --prefix frontend run check
npm --prefix frontend run build
```

CI 检查 Windows x64、ARM64 的构建、前端测试、Go 测试和 `go vet`。CD 在 Release 发布后构建并上传对应架构的 EXE，也支持在 Actions 页面手动运行并下载构建产物。

## 🔐 安全报告

如果发现安全问题，请不要公开披露细节。请优先参考仓库中的 [SECURITY.md](./SECURITY.md) 提交安全报告。

## 📄 许可证

本项目基于 [GPL-3.0](./LICENSE) 开源。

<div align="center">
  <sub>Built with ❤️ by Sunny</sub>
</div>
