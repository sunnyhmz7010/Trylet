---
name: Trylet
description: "随手打开的单文件练习窗口；Magpie 式浅灰框架、白色工作面与紧凑控件"
colors:
  primary: "#4f46e5"
  primary-foreground: "#ffffff"
  background: "#f4f4f6"
  card: "#ffffff"
  secondary: "#ededf1"
  foreground: "#1c1c21"
  muted: "#63636b"
  border: "#e3e3e8"
  active-fill: "#e9e7ff"
  destructive: "#c43535"
  success: "#17834b"
  primary-dark: "#a5a0ff"
  primary-foreground-dark: "#181426"
  background-dark: "#1a1a1e"
  card-dark: "#232327"
  secondary-dark: "#303035"
  foreground-dark: "#ededf1"
  muted-dark: "#b0b0ba"
  border-dark: "#393940"
  active-fill-dark: "#33314c"
  destructive-dark: "#ff9c9c"
  success-dark: "#6fd99b"
typography:
  title:
    fontFamily: '"Segoe UI", "Microsoft YaHei UI", sans-serif'
    fontSize: "16px"
    fontWeight: 600
    letterSpacing: "0"
  section:
    fontFamily: '"Segoe UI", "Microsoft YaHei UI", sans-serif'
    fontSize: "13px"
    fontWeight: 600
  body:
    fontFamily: '"Segoe UI", "Microsoft YaHei UI", sans-serif'
    fontSize: "13px"
    fontWeight: 400
  label:
    fontFamily: '"Segoe UI", "Microsoft YaHei UI", sans-serif'
    fontSize: "12px"
    fontWeight: 500
  caption:
    fontFamily: '"Segoe UI", "Microsoft YaHei UI", sans-serif'
    fontSize: "11px"
    fontWeight: 400
    lineHeight: 1.65
  code:
    fontFamily: '"Cascadia Code", Consolas, monospace'
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.7
  terminal:
    fontFamily: '"Cascadia Code", Consolas, monospace'
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.3
rounded:
  menu-item: "4px"
  toolbar: "5px"
  control: "6px"
  group: "7px"
  workspace: "9px"
  dialog: "12px"
  circle: "50%"
spacing:
  micro: "4px"
  compact: "6px"
  base: "8px"
  group: "10px"
  content: "12px"
  section: "16px"
  dialog-inset: "22px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.primary-foreground}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "0 11px"
    height: "30px"
  button-primary-hover:
    backgroundColor: "color-mix(in oklab, #4f46e5 90%, transparent)"
  button-outline:
    backgroundColor: "{colors.background}"
    textColor: "{colors.foreground}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "0 11px"
    height: "30px"
  button-outline-hover:
    backgroundColor: "{colors.secondary}"
  button-ghost:
    textColor: "{colors.foreground}"
    rounded: "{rounded.control}"
    height: "26px"
  button-ghost-hover:
    backgroundColor: "{colors.secondary}"
  button-destructive:
    backgroundColor: "{colors.destructive}"
    textColor: "{colors.primary-foreground}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "0 11px"
    height: "30px"
  button-link:
    textColor: "{colors.primary}"
    height: "26px"
  field-input:
    textColor: "{colors.foreground}"
    rounded: "{rounded.control}"
    padding: "3.25px 9.75px"
    height: "33px"
  app-navigation:
    backgroundColor: "{colors.background}"
    textColor: "{colors.muted}"
    padding: "5px 12px 0"
    height: "43px"
  folder-tab:
    backgroundColor: "{colors.secondary}"
    textColor: "{colors.muted}"
    rounded: "7px 7px 0 0"
    padding: "0"
    width: "49px"
    height: "31px"
  folder-tab-selected:
    backgroundColor: "{colors.card}"
    textColor: "{colors.primary}"
    height: "34px"
  toolbar:
    backgroundColor: "{colors.background}"
    textColor: "{colors.muted}"
    padding: "0 12px"
    height: "41px"
  settings-page:
    backgroundColor: "{colors.card}"
    textColor: "{colors.foreground}"
    rounded: "0 9px 9px 9px"
  runtime-card:
    rounded: "{rounded.group}"
    padding: "10px"
  dialog:
    backgroundColor: "{colors.background}"
    rounded: "{rounded.dialog}"
    width: "min(600px, calc(100vw - 32px))"
  floating-ball:
    backgroundColor: "{colors.background}"
    textColor: "{colors.primary}"
    rounded: "{rounded.circle}"
    padding: "7.5px"
    size: "48px"
---

# Design System: Trylet

## Overview

**Creative North Star: "随手打开的练习窗口"**

Trylet 是围绕单文件代码编辑与本地 Git 只读浏览的紧凑桌面工具，统一使用这一英文名，不设中文名。已确认的 Magpie 视觉承诺是浅灰框架、白色工作面、细边框、紧凑控件和克制阴影；文字、代码与操作本身构成界面，无需图片式装饰。

视觉重心留给代码。顶部以三个文件夹状、仅图标的标签切换编辑器、Git 与设置；语言和草稿留在编辑器页内工具栏，窗口操作在公共导航右侧。主要动作使用紫色，设置在第三个页面完成，发布继续使用对话框；两者都保留标题、可滚动正文和明确提交区。沿用 Windows 界面字体是操作界面的实际选择；它承担正文与标签，不作为展示型标题系统。

品牌采用用户批准的第一轮 A 图标：T 与向前箭头组成同一实心标志。紫色圆角方块白标用于应用识别，圆形悬浮球沿用浅灰底与随状态变色的标志；从悬浮球唤出、编辑并本机运行，到 Git 浏览与主动发布的既有操作路径不变。

**Key Characteristics:**

- 浅灰框架包住白色工作面，以细线建立层级。
- 小型圆角控件，公共导航采用仅图标的文件夹标签，主动作以紫色聚焦。
- 界面字体与等宽代码字体分工，密度服从短时编辑与运行。
- 悬浮球保持静止，仅在交互状态变化时反馈。
- Trylet 名称与 T 箭头统一品牌识别，方块应用图标和圆形悬浮球共用同一形状。

品牌标志以 `frontend/src/brand.svg` 为准，应用资源由 `build/make-icon.ps1` 共用该形状生成。


## Colors

主色是一种明确但克制的紫色，辅助层级主要由中性色承担。颜色值以 frontmatter 为准；暗色条目对应主窗口源码中随系统外观切换的同名语义角色。

### Primary

- **操作紫（primary）**：品牌标志、应用图标底色、运行、确认提交、链接、键盘焦点和编辑光标。
- **主动作文字（primary-foreground）**：实心主按钮上的文字与图标。
- **淡紫反馈（active-fill）**：置顶激活、悬浮球悬停及终端拖动条悬停。它是状态底色，不承担正文。

### Neutral

- **框架浅灰（background）**：窗口框架、路径预览及对话框正文。
- **工作面白（card）**：编辑区、终端、菜单与对话框操作区。
- **悬停灰（secondary）**：次级控件、菜单项和目录项的状态反馈。
- **近黑正文（foreground）**：正文与清晰的操作标签。
- **次级灰字（muted）**：环境信息、保存状态、说明与非激活图标。
- **细线灰（border）**：控件轮廓、工作区边界、对话框分区。

成功与失败分别使用 success、destructive，保持状态语义。暗色模式复用相同角色，悬浮球另有独立暗色描边和紫色细调；这些局部细调只属于悬浮球。

Git 内部的浅色、深色与绿色主题由 `.oil-git` 内的变量与状态控制，语言和布局偏好也限定于该区域；它们不是全应用主题或公共导航规范。

**The Action Color Rule.** 紫色用于品牌识别、可执行操作、焦点与已激活状态；环境就绪、失败信息继续使用各自的状态色。

## Typography

**Body Font:** Segoe UI，后接 Microsoft YaHei UI 与 sans-serif。
**Label/Mono Font:** 编辑区和终端使用 Cascadia Code、Consolas、monospace；路径和差异区域优先 Consolas。
**Character:** 界面文字保持 Windows 工具的熟悉感，代码以等宽节奏建立主体。没有展示型大标题或额外的品牌字体。

### Hierarchy

- **Title**：设置页与对话框标题，半粗、常规字距。
- **Section**：设置分区标题，和正文同尺寸、以字重区分。
- **Body**：应用根字号，也是代码区字号的基准。
- **Label**：语言选择、字段标签及底部主操作。
- **Caption**：说明、环境和保存状态；说明文本使用较松行高。
- **Code / Terminal**：编辑区行距更宽，终端使用更紧凑的等宽排版。

字号与行高见 frontmatter。日期、快捷键和部分路径目前存在更小的辅助文字，未将其扩成正文规范。Tailwind 的 rem 以应用根字号计算：通用按钮的 text-sm 并不等于固定的 14px；底部按钮有明确的像素尺寸覆盖。

**The Code First Rule.** 正文和工具文字服务代码阅读，不增加独立的展示标题来争夺工作区。

## Layout

主窗口初始尺寸（720×560px），最小尺寸（500×380px）；悬浮球窗口（48×48 DIP）。这是当前桌面表面的实现尺寸，不要求未来所有表面采用同一构图。

主窗口以垂直弹性布局排列公共导航和当前页面。公共导航高（43px），内边距（5px 12px 0），左侧是 editor / Git / settings 三枚文件夹标签，右侧是置顶与收起；标签规格见 app-navigation、folder-tab 和 folder-tab-selected。

编辑器页内的语言与草稿工具栏高（41px）、水平内边距（12px）。白色代码工作区在左右各留（10px）边距并占据剩余高度；编辑器底栏高（49px），左侧环境与保存状态，右侧发布与运行。Git 与设置页面直接占据公共导航下方的剩余空间，左右及底部留（10px）边距，页内内容独立滚动，不复用编辑器底栏。

Git 页保留 Oil Git 的页内项目、分支、刷新、主题与语言工具栏；更改列表位于左侧，差异或提交历史位于主区。主题、语言及布局变量作用于 `.oil-git`。设置页使用固定标题、独立滚动正文和固定底部操作区，宽度随页面展开，不受模态框的最大宽度限制。

终端在编辑器工作区下方展开，初始高（190px），通过顶部拖动条调整，并为代码区保留空间。发布等对话框限制在视口内，最大宽（600px）、宽度为视口减（32px），最大高为动态视口减（28px）；标题和操作区固定，正文独立滚动。设置页与常规对话框水平内边距（22px）。

窄于或等于（570px）时隐藏运行快捷键文字，缩短草稿、环境与状态文本的可用宽度，设置页与对话框的标题、正文水平内边距降至（16px）。公共导航保持三枚图标标签；最小窗口的编辑器、终端与 Git 已通过改名前的历史浅色截图核验，设置页与发布对话框在该尺寸下尚未单独验收。

间距以小步长组织，常用值见 frontmatter；少量（7px、9px）用于控件局部，不将其伪装成统一的四像素网格。

## Elevation & Depth

工作面在静止状态依靠色阶与一像素细线区分。轻微阴影只在原语控件及浮层上建立触感与覆盖关系；没有给整个代码工作面叠加强阴影。

### Shadow Vocabulary

- **Control**：输入框、选择框和描边按钮的轻微下缘，`0 1px 2px 0 #0000000d`。
- **Popover**：下拉菜单和选择菜单，`0 4px 6px -1px #0000001a, 0 2px 4px -2px #0000001a`。
- **Dialog**：对话框，`0 10px 15px -3px #0000001a, 0 4px 6px -4px #0000001a`；背景遮罩是半透明黑色。

**The Quiet Surface Rule.** 用灰白面和细线组织静止区域；阴影用于控件与浮层，不把每个分组都抬成卡片。

交互转换采用通用控件的（150ms）标准缓动，悬浮球背景与文字转换为（120ms）。系统请求减少动效时缩短或关闭转换。不存在悬浮球循环动画。

## Shapes

圆角随容器尺度递进：菜单项、工具按钮、标准控件、设置分组、代码工作面和对话框各有职责，数值见 frontmatter。悬浮球是唯一圆形入口；普通操作继续使用小圆角矩形。

公共导航标签以上方凸起的文件夹耳部形成轮廓，耳部宽（22px）、高（5px）；选中标签增高并使用白色底和紫色图标，不增加阴影。Git 与设置的页面容器左上角平直，其余角延续工作面的圆角。

细边框是这个视觉世界的原生材料。编辑器行号区域没有额外竖线；对话框标题、正文、操作区及目录面包屑使用分隔线。功能图标来自 Lucide 的 SVG 线形图标，运行和停止使用明确的形状反馈。

品牌标志使用（32×32）viewBox 内的单一 polygon，将 T 与向前箭头连接；glyph 继承 currentColor。应用图标使用同一 polygon，在（32×32）画布上以（7.36）圆角的紫色方块承载白标，标志按（0.75）缩放并平移（4, 4）。这些数值只描述品牌资源，不扩成公共容器圆角或间距。悬浮球继续使用既有圆形外壳。

## Components

### Buttons

紧凑、明确，保持操作工具的密度。

- **Primary**：底部运行、设置页保存以及对话框确认使用实心操作紫；底部尺寸见 button-primary。
- **Outline**：发布、取消及次要操作使用细线和灰色底，悬停进入次级底色。
- **Ghost / Link**：重新检测、轻量取消与创建令牌入口减少容器强调，链接使用主色和悬停下划线。
- **Destructive**：运行中切换为停止按钮；停止与失败信息使用危险色。
- **Focus / Disabled**：按钮和输入有可见焦点外轮廓，原语另有三像素半透明焦点环；禁用原语降低透明度并关闭交互。

### Cards / Containers

设置中的运行环境组是描边分组，内边距与圆角见 runtime-card，无独立抬升阴影。主代码容器使用工作面白，避免把每段设置或终端输出做成独立装饰卡片。

### Inputs / Fields

字段采用细边框、小圆角、透明底色；设置页与发布对话框字段高（33px）、标签与字段间距（7px），原生运行环境/分支选择框高（32px）。Input 原语的焦点使用主色轮廓与焦点环；错误在字段附近用危险色文字表达，禁用状态保留当前值。路径预览是浅灰辅助面，长路径允许换行。

### Navigation

公共导航使用三个文件夹状、仅图标的编辑器、Git、设置标签，默认灰字灰底，悬停转为较浅底色，选中为白底和操作紫。图标保留可访问名称与标题提示，标签组支持方向键、Home、End 切换，并显示键盘焦点。置顶与收起位于右侧；置顶激活有淡紫底和紫色图标。

语言触发器和草稿菜单仅属于编辑器页内工具栏：语言为白底描边控件，草稿以灰字和下箭头表达。草稿历史菜单有独立滚动范围，长名称截断，时间退居辅助层。Git 页的更改侧栏及差异/历史浏览属于其浏览工作区，不扩展为全应用侧边导航。

### Settings Page

设置位于第三个页面，直接使用白色工作面与细线分区；标题、滚动正文和底部取消/保存操作延续现有字段与按钮原语，不使用模态遮罩、浮层阴影或关闭图标。取消恢复已保存设置并返回编辑器；保存结果留在相关字段和操作区附近。

### Dialogs

发布继续使用标题、滚动正文与底部操作区的模态结构。发布目录使用面包屑、可滚动文件夹列表和最终路径预览；冲突时展示左右差异并保留“另存为新文件”与更新操作。对话框的这些内容决定信息密度，不构成第二套视觉主题；设置页不再使用此模态外壳。

### Floating Ball

静止的圆形代码入口，描边、浅灰底和紫色 T 箭头实心标志；尺寸与内边距见 floating-ball。内边距从（11px）调整为（7.5px），匹配批准的 A 方案预览；glyph 继承当前文字色，沿用既有悬停、按下和暗色状态。悬停和按下通过淡紫底色变化反馈，键盘焦点加粗主色边框；不靠持续动画吸引注意。原生窗口保留不透明背景，通过椭圆窗口区域呈现圆形；改名前的单显示器四档缩放曾核验该外形，前端视口维持（48×48px），对应原生物理外接矩形为（48×48、60×60、72×72、96×96px）；此历史范围不代表新标志在四档缩放下已重新验收。

### Brand Assets

Trylet 是主窗口、托盘、EXE 与启动状态的统一名称；启动状态使用生成的 `build/icon.svg`，以（32×32px）显示同一应用图标。`build/make-icon.ps1` 从权威 glyph 生成 `build/icon.svg`、`build/icon.png` 和 `build/icon.ico`，ICO 包含（16、20、24、32、40、48、64、128、256px）九档资源。品牌资源保持方块白标与圆球着色 glyph 两种承载方式，共用批准的 A 方案形状。

## Do's and Don'ts

### Do:

- **Do** 延续浅灰框架、白色工作面和细边框的 Magpie 视觉承诺。
- **Do** 用主色、状态色及明确文字分别表达操作、就绪和失败。
- **Do** 保留紧凑工具栏与宽松代码行距之间的层级。
- **Do** 复用字段、对话框和轻量菜单，让状态变化留在相关控件附近。
- **Do** 保留键盘焦点与减少动效的系统偏好。
- **Do** 统一使用 Trylet 英文名及批准的 A 版 T 箭头，共用权威 glyph 生成品牌资源。

### Don't:

- **Don't** 用额外展示标题或图片装饰挤压单文件编辑的主体。
- **Don't** 给所有分组增加阴影或把普通按钮改成圆形入口。
- **Don't** 将尚未视觉核验的深色代码色、极小辅助文字或浮层动画当作新表面的默认规范。

未纳入规范、未修复的实现偏差：10px 辅助文字在未覆盖场景下的可读性与 CodeMirror 深色模式中固定浅色值仍待视觉核验；组件源码包含 fade/zoom/animate-in 类，但当前构建 CSS 未产出对应规则，只有 spin 关键帧，因此不将淡入缩放记作已实现动效。此轮只同步记录，未修改这些行为。
