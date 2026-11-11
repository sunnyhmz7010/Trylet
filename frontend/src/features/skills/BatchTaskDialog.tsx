import { useEffect, useRef } from 'react'
import { Button } from '../../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../../components/ui/dialog'
import type { BatchState, SkillBatchTask } from './batchTask'
function ResultMessage({ message }: { message: string }) {
  const summary = message.split('\n')[0].slice(0, 100)
  return summary === message ? <p>{message}</p> : <details className="skill-issue"><summary>{summary}…</summary><p>{message}</p></details>
}
const names = { pending: '等待', running: '处理中', success: '成功', failed: '失败', skipped: '跳过', unexecuted: '未执行' }
export default function BatchTaskDialog({ task, state }: { task: SkillBatchTask; state: BatchState }) {
  const safe = useRef<HTMLButtonElement>(null)
  const done = state.phase === 'done', conflict = state.phase === 'confirmation'
  useEffect(() => { if (conflict) safe.current?.focus() }, [conflict, state.conflict?.confirmation])
  const current = state.results[state.current]
  const failures = state.results.filter((item) => item.status === 'failed').length
  const complete = state.results.filter((item) => item.status !== 'pending' && item.status !== 'running' && item.status !== 'unexecuted').length
  const action = state.kind === 'update' ? '更新' : '删除'
  return <Dialog open={state.phase !== 'idle'} onOpenChange={(open) => { if (!open && done) task.close() }}>
    <DialogContent data-operation-busy={!done} size="md" className="dialog-content" showCloseButton={done} onOpenAutoFocus={(event) => { event.preventDefault(); safe.current?.focus() }} onEscapeKeyDown={(event) => { if (!done) event.preventDefault() }}>
      <div className="dialog-heading"><DialogTitle>{conflict ? `确认覆盖：${current?.item.name}` : `批量${action}${done ? '结果' : ''}`}</DialogTitle>
        <DialogDescription>{conflict ? '此安装项存在本地改动，覆盖会替换安装目录。请逐项决定，跳过会继续处理下一项。' : done ? `已处理 ${complete} / ${state.results.length} 项` : `正在处理 ${state.current + 1} / ${state.results.length} 项：${current?.item.name ?? ''}`}</DialogDescription></div>
      <div className="dialog-scroll batch-task-body">
        {conflict ? <><p className="skill-full-path">{current?.item.path}</p><ul className="skill-changes">{state.conflict?.changes?.map((path) => <li key={path}>{path}</li>)}</ul></> : <>
          <progress aria-label="批量任务进度" max={state.results.length || 1} value={complete}/>
          {done && <p className="batch-summary">{(['success','failed','skipped','unexecuted'] as const).map((status) => `${names[status]} ${state.results.filter((item) => item.status === status).length}`).join(' · ')}</p>}
          <ul className="batch-results">{state.results.map(({ item, status, message, warning }) => <li key={item.id}><div><strong>{item.name}</strong><span data-status={status}>{names[status]}</span></div>{message && <ResultMessage message={message}/>}{warning && <div className="batch-warning"><ResultMessage message={`警告：${warning}`}/></div>}</li>)}</ul>
        </>}
      </div>
      <div className="dialog-footer">
        {conflict ? <><Button variant="ghost" onClick={() => task.stop()}>停止批量更新</Button><Button ref={safe} variant="outline" onClick={() => task.decide('skip')}>跳过此项</Button><Button variant="destructive" onClick={() => task.decide('overwrite')}>覆盖并继续</Button></> : done ? <><Button ref={safe} variant="outline" onClick={() => task.close()}>关闭</Button>{failures > 0 && <Button onClick={() => task.retry()}>重试失败项（{failures}）</Button>}</> : <Button ref={safe} variant="outline" disabled={state.stopping} onClick={() => task.stop()}>{state.stopping ? '当前项完成后停止…' : '停止后续任务'}</Button>}
      </div>
    </DialogContent>
  </Dialog>
}
