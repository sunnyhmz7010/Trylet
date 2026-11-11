import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useWorkspaceActions } from './useWorkspaceActions'
const mocks = vi.hoisted(() => ({ quit: vi.fn(async () => {}), remember: vi.fn(async () => ({})), showCloseError: vi.fn() }))
vi.mock('react', () => ({ useState: (value: unknown) => [value, vi.fn()] }))
vi.mock('./useWorkspaceShortcuts', () => ({ useWorkspaceShortcuts: vi.fn() }))
vi.mock('./useWindowActions', () => ({ useWindowActions: () => ({ showCloseError: mocks.showCloseError }) }))
vi.mock('../../bindings/trylet/internal/desktop/desktop', () => ({ Quit: mocks.quit, RememberCloseAction: mocks.remember }))
vi.mock('../../bindings/trylet/internal/app/service', () => ({ StopRun: vi.fn(async () => {}), NewDraft: vi.fn(), LoadDraft: vi.fn() }))
let busy = false
beforeEach(() => { busy = false; vi.clearAllMocks(); vi.stubGlobal('document', { querySelector: () => busy ? {} : null }) })
afterEach(() => vi.unstubAllGlobals())
function setup() {
  const flush = vi.fn(async () => undefined), setError = vi.fn(), setSettings = vi.fn()
  const actions = useWorkspaceActions({ transitionLock: { current: false }, flush, hydrateDraft: vi.fn(), refreshHistory: vi.fn(async () => {}) }, { phaseRef: { current: 'idle' }, activeRun: { current: '' }, run: vi.fn(async () => {}), reset: vi.fn() }, { publish: vi.fn(async () => {}) }, null, setSettings, setError, 'editor')
  return { actions, flush, setError }
}
describe('维护任务退出保护', () => {
  it('后台页签的维护任务执行期间拒绝退出，不保存关闭偏好', () => {
    const { actions, flush, setError } = setup(); busy = true
    actions.requestAction({ kind: 'quit', rememberClose: true })
    expect(flush).not.toHaveBeenCalled(); expect(mocks.remember).not.toHaveBeenCalled(); expect(mocks.quit).not.toHaveBeenCalled()
    expect(setError).toHaveBeenCalledWith(expect.stringContaining('当前任务尚未完成'))
  })
  it('任务完成后先保存，再记住关闭选择并退出', async () => {
    const { actions, flush } = setup()
    actions.requestAction({ kind: 'quit', rememberClose: true })
    await vi.waitFor(() => expect(mocks.quit).toHaveBeenCalledOnce())
    expect(flush).toHaveBeenCalledOnce(); expect(mocks.remember).toHaveBeenCalledWith('quit')
    expect(flush.mock.invocationCallOrder[0]).toBeLessThan(mocks.remember.mock.invocationCallOrder[0])
    expect(mocks.remember.mock.invocationCallOrder[0]).toBeLessThan(mocks.quit.mock.invocationCallOrder[0])
  })
  it('保存失败不改变关闭偏好，也不会退出', async () => {
    const { actions, flush } = setup(); flush.mockRejectedValueOnce(new Error('磁盘写入失败'))
    actions.requestAction({ kind: 'quit', rememberClose: true })
    await vi.waitFor(() => expect(mocks.showCloseError).toHaveBeenCalled())
    expect(mocks.remember).not.toHaveBeenCalled(); expect(mocks.quit).not.toHaveBeenCalled()
  })
})
