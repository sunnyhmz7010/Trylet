import { describe, expect, it } from 'vitest'
import { PublishContinuation } from './publishContinuation'

describe('补充设置后继续发布', () => {
  it('原草稿配置完整后仅恢复一次', () => {
    const flow = new PublishContinuation()
    flow.request('draft-a')
    expect(flow.consume('draft-a', true)).toBe(true)
    expect(flow.consume('draft-a', true)).toBe(false)
  })
  it('配置不完整时保留原草稿的发布意图', () => {
    const flow = new PublishContinuation()
    flow.request('draft-a')
    expect(flow.consume('draft-a', false)).toBe(false)
    expect(flow.consume('draft-a', true)).toBe(true)
  })
  it('主动切页取消后保存设置不触发发布', () => {
    const flow = new PublishContinuation()
    flow.request('draft-a')
    flow.cancel()
    expect(flow.consume('draft-a', true)).toBe(false)
  })
  it('切换草稿使旧意图失效，切回也不恢复', () => {
    const flow = new PublishContinuation()
    flow.request('draft-a')
    expect(flow.consume('draft-b', true)).toBe(false)
    expect(flow.consume('draft-a', true)).toBe(false)
  })
})
