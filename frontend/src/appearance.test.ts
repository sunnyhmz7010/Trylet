import { afterEach, expect, it, vi } from 'vitest'
import { watchAppearance } from './appearance'

afterEach(() => vi.unstubAllGlobals())

it('全局模式覆盖系统偏好，跟随系统时响应变化，卸载后停止监听', () => {
  const media = new EventTarget() as EventTarget & { matches: boolean }
  media.matches = true
  const root = { dataset: { theme: '' }, style: { colorScheme: '' } }
  vi.stubGlobal('matchMedia', () => media)
  vi.stubGlobal('document', { documentElement: root })
  vi.stubGlobal('window', new EventTarget())
  let cleanup = watchAppearance('light')
  expect(root.dataset.theme).toBe('light')
  media.dispatchEvent(new Event('change'))
  expect(root.dataset.theme).toBe('light')
  cleanup()
  cleanup = watchAppearance('system')
  expect(root.dataset.theme).toBe('dark')
  media.matches = false
  media.dispatchEvent(new Event('change'))
  expect(root.dataset.theme).toBe('light')
  cleanup()
  media.matches = true
  media.dispatchEvent(new Event('change'))
  expect(root.dataset.theme).toBe('light')
})
