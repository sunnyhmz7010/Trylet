import { describe, expect, it, vi } from 'vitest'
import { SkillBatchTask, reconcileSelection, updateUnavailable } from './batchTask'
import type { SkillInstallation, SkillUpdateResult } from '../../../bindings/trylet/internal/skills/models'
const item = (id: string, extra: Partial<SkillInstallation> = {}): SkillInstallation => ({ id, name: id, repository: 'demo/skills', locationID: 'root', path: `/skills/${id}`, mode: 'extracted', readOnly: false, linked: false, missing: false, ...extra } as SkillInstallation)
const result = (extra: Partial<SkillUpdateResult> = {}): SkillUpdateResult => ({ needsConfirmation: false, message: '已更新', warning: '', confirmation: '', changes: [], ...extra })
const done = async (task: SkillBatchTask) => vi.waitFor(() => expect(task.getSnapshot().phase).toBe('done'))
describe('Skills 批量任务', () => {
  it('拒绝只读选择，并在刷新后删除不存在的选择', () => {
    expect(reconcileSelection(['a','b','c'], [item('a'),item('b',{readOnly:true})])).toEqual(['a'])
    expect(updateUnavailable(item('a',{linked:true}))).not.toBe('')
    expect(updateUnavailable(item('a',{repository:''}))).not.toBe('')
    expect(updateUnavailable(item('a',{mode:'git',missing:true}))).not.toBe('')
  })
  it('串行固定目标快照，重复启动无效', async () => {
    let release!: (value: SkillUpdateResult) => void
    const update = vi.fn().mockImplementationOnce(() => new Promise((resolve) => { release=resolve })).mockResolvedValue(result())
    const task = new SkillBatchTask({update,remove:vi.fn()})
    const targets=[item('a'),item('b')]
    task.start('update',targets); targets[1].id='changed'; task.start('update',[item('extra')])
    expect(update).toHaveBeenCalledTimes(1)
    release(result()); await done(task)
    expect(update.mock.calls.map(([id])=>id)).toEqual(['a','b'])
  })
  it('跳过不可更新项，单项失败后继续并保留警告', async () => {
    const update=vi.fn().mockRejectedValueOnce(new Error('离线')).mockResolvedValue(result({warning:'备份保留'}))
    const task=new SkillBatchTask({update,remove:vi.fn()})
    task.start('update',[item('local',{repository:''}),item('a'),item('b')]); await done(task)
    expect(task.getSnapshot().results.map((entry)=>entry.status)).toEqual(['skipped','failed','success'])
    expect(task.getSnapshot().results[2].warning).toBe('备份保留')
    task.retry(); await done(task)
    expect(update.mock.calls.map(([id])=>id)).toEqual(['a','b','a'])
  })
  it('覆盖令牌变化时再次暂停确认，不复用旧确认', async () => {
    const update=vi.fn().mockResolvedValueOnce(result({needsConfirmation:true,confirmation:'first'})).mockResolvedValueOnce(result({needsConfirmation:true,confirmation:'second'})).mockResolvedValue(result())
    const task=new SkillBatchTask({update,remove:vi.fn()})
    task.start('update',[item('a')])
    await vi.waitFor(()=>expect(task.getSnapshot().conflict?.confirmation).toBe('first'))
    task.decide('overwrite')
    await vi.waitFor(()=>expect(task.getSnapshot().conflict?.confirmation).toBe('second'))
    expect(update).toHaveBeenCalledTimes(2)
    task.decide('overwrite'); await done(task)
    expect(update.mock.calls).toEqual([['a',''],['a','first'],['a','second']])
  })
  it('冲突跳过后继续下一项', async () => {
    const update=vi.fn().mockResolvedValueOnce(result({needsConfirmation:true,confirmation:'token'})).mockResolvedValue(result())
    const task=new SkillBatchTask({update,remove:vi.fn()}); task.start('update',[item('a'),item('b')])
    await vi.waitFor(()=>expect(task.getSnapshot().phase).toBe('confirmation')); task.decide('skip'); await done(task)
    expect(task.getSnapshot().results.map((entry)=>entry.status)).toEqual(['skipped','success'])
  })
  it('确认时停止保留本地内容，后续项不执行', async () => {
    const update=vi.fn().mockResolvedValue(result({needsConfirmation:true,confirmation:'token'}))
    const task=new SkillBatchTask({update,remove:vi.fn()}); task.start('update',[item('a'),item('b')])
    await vi.waitFor(()=>expect(task.getSnapshot().phase).toBe('confirmation')); task.stop(); await done(task)
    expect(task.getSnapshot().results.map((entry)=>entry.status)).toEqual(['unexecuted','unexecuted'])
    expect(update).toHaveBeenCalledTimes(1)
  })
  it('删除过程中停止只阻止后续项，不中断当前写入', async () => {
    let release!: (value:string)=>void
    const remove=vi.fn().mockImplementation(()=>new Promise((resolve)=>{release=resolve}))
    const task=new SkillBatchTask({update:vi.fn(),remove}); task.start('delete',[item('a'),item('b')]); task.stop()
    expect(task.getSnapshot().phase).toBe('running'); release(''); await done(task)
    expect(task.getSnapshot().results.map((entry)=>entry.status)).toEqual(['success','unexecuted'])
    expect(remove).toHaveBeenCalledTimes(1)
  })
  it('删除失败不阻断其他项，链接与缺失记录使用已有删除接口', async () => {
    const remove=vi.fn().mockRejectedValueOnce(new Error('占用')).mockResolvedValue('')
    const task=new SkillBatchTask({update:vi.fn(),remove}); task.start('delete',[item('a'),item('b',{linked:true}),item('c',{missing:true})]); await done(task)
    expect(task.getSnapshot().results.map((entry)=>entry.status)).toEqual(['failed','success','success'])
    expect(remove.mock.calls).toEqual([['root','a'],['root','b'],['root','c']])
  })
})
