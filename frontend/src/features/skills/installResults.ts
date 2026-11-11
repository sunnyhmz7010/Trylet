import type { SkillInstallResult } from '../../../bindings/trylet/internal/skills/models'

// 后端遗漏结果时不能以空数组 every() 判定全部成功，更不能重复安装成功项。
export function normalizeInstallResults(paths: string[], results: SkillInstallResult[]): SkillInstallResult[] {
  return paths.map((path) => results.find((result) => result.sourcePath === path) ?? {
    sourcePath: path, error: '未收到此项的安装结果，请刷新列表核对后重试。', warning: '', name: path, path: '',
  })
}
