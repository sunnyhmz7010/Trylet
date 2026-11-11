import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useWorkspaceShortcuts } from './useWorkspaceShortcuts'
const effects = vi.hoisted(() => [] as (() => void | (() => void))[])
vi.mock('react', () => ({ useRef: (current: unknown) => ({ current }), useEffect: (effect: () => void | (() => void)) => effects.push(effect) }))
let target: EventTarget
let cleanup: void | (() => void)
let modal = false
const actions = { activeView: 'editor' as const, run: vi.fn(async () => {}), create: vi.fn(), publish: vi.fn(async () => {}), save: vi.fn(async () => {}), hide: vi.fn(async () => {}) }
function press(key: string, options: Record<string, unknown> = {}) {
  const event = new Event('keydown', { cancelable: true })
  Object.defineProperties(event, Object.fromEntries(Object.entries({ key, ctrlKey: true, shiftKey: false, altKey: false, metaKey: false, repeat: false, isComposing: false, keyCode: 0, ...options }).map(([name, value]) => [name, { value }])))
  target.dispatchEvent(event)
  return event
}
beforeEach(() => {
  vi.clearAllMocks(); modal = false; target = new EventTarget()
  vi.stubGlobal('window', target)
  vi.stubGlobal('document', { querySelector: () => modal ? {} : null })
  useWorkspaceShortcuts(actions); cleanup = effects.pop()!()
})
afterEach(() => { cleanup?.(); vi.unstubAllGlobals() })
describe('工作区快捷键保护', () => {
  it('运行命令先阻止编辑器默认处理，捕获和冒泡不会执行两次', () => {
    expect(press('Enter').defaultPrevented).toBe(true)
    expect(actions.run).toHaveBeenCalledOnce()
  })
  it('保存与发布各自只执行一次', () => {
    press('s'); press('s', { shiftKey: true })
    expect(actions.save).toHaveBeenCalledOnce(); expect(actions.publish).toHaveBeenCalledOnce()
  })
  it.each([{ isComposing: true }, { keyCode: 229 }, { repeat: true }, { altKey: true }, { metaKey: true }])('忽略输入法、按住重复及其他修饰键 %j', (options) => {
    expect(press('Enter', options).defaultPrevented).toBe(false)
    expect(actions.run).not.toHaveBeenCalled()
  })
  it('弹窗打开时不触发后台命令', () => {
    modal = true; press('Enter'); press('n'); press('s')
    expect(actions.run).not.toHaveBeenCalled(); expect(actions.create).not.toHaveBeenCalled(); expect(actions.save).not.toHaveBeenCalled()
  })
  it('卸载后移除捕获和冒泡监听', () => {
    cleanup?.(); press('Enter'); expect(actions.run).not.toHaveBeenCalled()
  })
})
