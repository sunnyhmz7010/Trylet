import { describe, expect, it, vi } from 'vitest'
import { performDraftTransition } from './draftTransition'

function setup() {
  const calls: string[] = []
  const step = (name: string) => vi.fn(async () => { calls.push(name); return name })
  return { calls, operations: {
    save: step('save'), stop: step('stop'), quit: step('quit'), create: step('new'),
    load: vi.fn(async (id: string) => { calls.push(`load:${id}`); return id }),
    activate: vi.fn((draft: string) => { calls.push(`activate:${draft}`) }), refresh: step('refresh'),
  } }
}
describe('草稿切换与退出', () => {
  it('先保存并停止，再载入目标草稿、更新界面和历史', async () => {
    const { calls, operations } = setup()
    await performDraftTransition({ kind: 'load', id: 'target' }, operations)
    expect(calls).toEqual(['save', 'stop', 'load:target', 'activate:target', 'refresh'])
  })
  it('新建走相同的保存保护', async () => {
    const { calls, operations } = setup()
    await performDraftTransition({ kind: 'new' }, operations)
    expect(calls).toEqual(['save', 'stop', 'new', 'activate:new', 'refresh'])
  })
  it('退出不创建或替换草稿', async () => {
    const { calls, operations } = setup()
    await performDraftTransition({ kind: 'quit' }, operations)
    expect(calls).toEqual(['save', 'stop', 'quit'])
  })
  it.each(['save', 'stop', 'load'] as const)('%s 失败会中断后续切换', async (stage) => {
    const { operations } = setup()
    operations[stage].mockRejectedValueOnce(new Error(stage))
    await expect(performDraftTransition({ kind: 'load', id: 'target' }, operations)).rejects.toThrow(stage)
    expect(operations.activate).not.toHaveBeenCalled()
    expect(operations.refresh).not.toHaveBeenCalled()
    if (stage === 'save') expect(operations.stop).not.toHaveBeenCalled()
  })
  it('等待保存完成后才停止任务', async () => {
    const { operations } = setup()
    let release!: (value: string) => void
    operations.save.mockImplementationOnce(() => new Promise((resolve) => { release = resolve }))
    const pending = performDraftTransition({ kind: 'quit' }, operations)
    expect(operations.stop).not.toHaveBeenCalled()
    release('saved')
    await pending
    expect(operations.quit).toHaveBeenCalledOnce()
  })
})
