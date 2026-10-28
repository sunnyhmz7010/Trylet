# Trylet

<!-- impeccable:product-schema 1 -->

## Platform
web

Windows 10/11 x64 桌面应用，使用系统 WebView2。

## Stack
用户确认：Wails 3、Go、React、TypeScript、Vite、CodeMirror 6、按需 shadcn/ui、xterm.js 和 ConPTY。

## Users
跟随课程练习 C、C++、Python 的学习者。

## Product Purpose
通过悬浮球快速打开单文件草稿，使用本机环境运行，再提交到指定 GitHub 仓库；在同一窗口只读浏览本地 Git 仓库。

## Capabilities and Constraints
自动保存与轻量历史；完整单文件与可关闭的空白模板；交互输入；可视化仓库目录；同名不同内容展示差异并让用户选择。顶部以三个文件夹状、仅图标的 editor / Git / settings 标签切换页面，语言与草稿位于编辑器页内工具栏，设置为第三个页面，发布继续使用对话框。Git 页面整合 Oil Git 的本地只读浏览，展示更改、差异、提交历史及分支与标签；界面仅提供中文。全局外观在设置页选择跟随系统、浅色或深色，Git 不再提供独立主题或语言切换。差异展示在设置页统一选择左右对照或统一差异，对所有仓库的工作区和提交历史生效，面板宽度不足时自动使用统一差异。设置分为常规、运行环境、Git 浏览和 GitHub 发布。默认失焦保持窗口、置顶关闭。源码 UTF-8、LF。不打包语言运行环境，不自动安装用户代码依赖。草稿、设置、Git 最近项目与偏好合并存入 EXE 同级数据文件，令牌以当前 Windows 账户加密后存入同一文件。临时缓存和运行产物仅使用同级目录，正常退出后清理，不在其他目录创建本应用的数据。

应用数据只保存在 EXE 同级 Trylet.json，运行缓存只使用同级 .Trylet-runtime，退出后清理。

## Brand Commitments
用户指定 Magpie 为参考：浅灰底、白色内容、细边框、紧凑控件、克制阴影；用户任务为 Operate。

产品统一使用英文名 Trylet，不设中文名。图标采用 2026-10-28 品牌探索第一轮 A 方案：T 与向前箭头组成实心标志；应用、启动状态、托盘和 EXE 共用紫色圆角方块白标，既有 48 DIP 圆形悬浮球使用随状态变色的同一标志。权威形状为 frontend/src/brand.svg，应用图标由 build/make-icon.ps1 从该形状生成。

## Product Principles
- 打开即可编辑，单文件为中心。
- 保存、运行与发布各有明确结果。
- 已有代码和失败时的用户输入不能丢失。
- 发布由用户主动触发，覆盖前展示差异。
