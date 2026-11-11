import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { NoticeStore, noticeDuration, type NoticeKind } from './noticeStore'
beforeEach(()=>vi.useFakeTimers())
afterEach(()=>vi.useRealTimers())
describe('临时通知',()=>{
  it.each(['success','warning','error'] as NoticeKind[])('%s 在规定时间自动消失',kind=>{
    const store=new NoticeStore(); store.notify('操作结果',kind)
    vi.advanceTimersByTime(noticeDuration[kind]-1); expect(store.getSnapshot()).toHaveLength(1)
    vi.advanceTimersByTime(1); expect(store.getSnapshot()).toHaveLength(0)
  })
  it('悬停、聚焦、失焦可叠加暂停，全部恢复后继续剩余时间',()=>{
    const store=new NoticeStore(); store.notify('失败','error'); vi.advanceTimersByTime(2000)
    store.pause('hover'); store.pause('focus'); store.pause('window'); vi.advanceTimersByTime(20000)
    store.resume('hover'); store.resume('focus'); vi.advanceTimersByTime(20000); expect(store.getSnapshot()).toHaveLength(1)
    store.resume('window'); vi.advanceTimersByTime(5999); expect(store.getSnapshot()).toHaveLength(1)
    vi.advanceTimersByTime(1); expect(store.getSnapshot()).toHaveLength(0)
  })
  it('相同消息合并并重新计时，旧关闭事件不能关闭新通知',()=>{
    const store=new NoticeStore(); store.notify('已保存'); const old=store.getSnapshot()[0]
    vi.advanceTimersByTime(2000); store.notify('已保存'); expect(store.getSnapshot()).toHaveLength(1)
    store.dismiss(old.id,old.revision); vi.advanceTimersByTime(2000); expect(store.getSnapshot()).toHaveLength(1)
    vi.advanceTimersByTime(1000); expect(store.getSnapshot()).toHaveLength(0)
  })
  it('最多三条，移除旧计时器；暂停时新通知不会过期',()=>{
    const store=new NoticeStore(); store.pause('window')
    for(const text of ['a','b','c','d']) store.notify(text)
    expect(store.getSnapshot().map(item=>item.message)).toEqual(['b','c','d'])
    vi.advanceTimersByTime(10000); expect(store.getSnapshot()).toHaveLength(3)
    store.resume('window'); vi.advanceTimersByTime(3000); expect(store.getSnapshot()).toHaveLength(0)
  })
})
