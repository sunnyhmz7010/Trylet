type Values = Record<string, string | number>
export function text(key: string, values?: Values): string {
  return key.replace(/\{([^{}]+)\}/g, (token, name: string) =>
    values && Object.prototype.hasOwnProperty.call(values, name) ? String(values[name]) : token)
}

const errorMessages: Record<string, string> = {
  desktopRequired: "请在 Trylet 桌面应用中打开。本地仓库读取需要桌面环境。",
  arguments: "读取参数无效，请重新打开仓库。",
  bare: "裸仓库暂不支持。请打开带工作目录的 Git 项目。",
  binary: "此文件不能按 Git 文本行查询作者。",
  changed: "文件在读取期间发生变化，请重试。",
  changing: "Git 正在变化，稍后自动重新读取。",
  config: "应用配置读取失败。",
  conflict: "冲突解决前无法确定行归属。",
  git: "Git 读取失败。",
  gitMissing: "没有找到可用的 Git，请重新检测。",
  gitUnavailable: "无法启动 Git。请检查 Git 安装并重试。",
  invalid: "Git 读取请求无效。",
  io: "文件系统读取失败。",
  lfsAttributeSourceUnsupported:
    "Git 无法读取历史 LFS 属性。原始 Git 差异仍可查看。",
  lfsHelper: "无法安全定位 Git LFS helper。",
  missing: "无法找到请求的项目或对象。",
  missingReference: "这个分支或标签已被移除。",
  notRepository:
    "这个文件夹尚未初始化 Git。请在编辑器或终端中初始化后重新打开。",
  open: "无法打开相关链接。",
  partialCloneUnsupported:
    "只读模式暂不支持部分克隆仓库，避免按需下载 Git 对象。",
  path: "项目路径无效。",
  process: "Git 子进程发生错误。请重试。",
  read: "读取失败。请重试。",
  recent: "无法读取最近项目记录。",
  session: "项目已关闭，请重新打开。",
  staleHistory: "提交历史已变化，正在重新加载。",
  symlink: "符号链接内容不按普通文本文件读取。",
  timeout: "读取超时",
  tooLarge: "读取结果过大，无法完整显示。",
  unsafeFilter: "Git 文件过滤器不受支持，无法安全读取。",
  unsafePath: "项目包含无法安全读取的路径。",
  unsupportedLfsExtension: "暂不支持含 Git LFS 扩展字段的指针文件。",
  watch: "无法监听仓库变化。",
};

type GitError = { kind?: string; messageKey?: string; message?: string }
export function errorSummary(error?: GitError | null): string {
  if (!error) return ''
  return errorMessages[error.messageKey ?? error.kind ?? ''] ?? '读取失败。请重试。'
}
export function errorDiagnostic(error?: GitError | null): string | null {
  if (!error?.message) return null
  return errorSummary(error) === error.message ? null : error.message
}
