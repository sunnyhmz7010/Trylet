import { describe, expect, it } from 'vitest'
import { normalizeInstallResults } from './installResults'
import type { SkillInstallResult } from '../../../bindings/trylet/internal/skills/models'
const result = (sourcePath: string, error = '', warning = ''): SkillInstallResult => ({sourcePath, name: sourcePath, path: '/test', error, warning})
describe('安装结果核对', () => {
  it('空响应不能显示安装成功', () => {
    const results = normalizeInstallResults(['a', 'b'], [])
    expect(results).toHaveLength(2)
    expect(results.every((item) => !!item.error)).toBe(true)
  })
  it('部分成功保留原结果，遗漏项标为失败', () => {
    const success = result('a')
    const results = normalizeInstallResults(['a', 'b'], [success])
    expect(results[0]).toBe(success)
    expect(results[1].sourcePath).toBe('b')
    expect(results[1].error).not.toBe('')
  })
  it('重试仅核对本次请求，不带入已安装项目', () => {
    expect(normalizeInstallResults(['b'], [result('a'), result('b')]).map((item) => item.sourcePath)).toEqual(['b'])
  })
  it('保留错误和清理警告，不将其伪装成干净成功', () => {
    const failed = result('a', '目录被占用')
    const warning = result('b', '', '旧备份清理失败')
    expect(normalizeInstallResults(['a', 'b'], [failed, warning])).toEqual([failed, warning])
  })
})
