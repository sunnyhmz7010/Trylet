import type { SkillInstallation, SkillUpdateResult } from '../../../bindings/trylet/internal/skills/models'

export function updateUnavailable(item: SkillInstallation): string {
  if (item.readOnly) return '系统内置，只读'
  if (item.linked) return '链接目录，请在原始位置更新'
  if (!item.repository) return '未绑定来源仓库'
  if (item.mode === 'git' && item.missing) return 'Git 目录已不存在'
  return ''
}
export function reconcileSelection(ids: string[], items: SkillInstallation[]) {
  const eligible = new Set(items.filter((item) => !item.readOnly).map((item) => item.id))
  return ids.filter((id) => eligible.has(id))
}
export type BatchKind = 'update' | 'delete'
export type BatchResult = { item: SkillInstallation; status: 'pending' | 'running' | 'success' | 'failed' | 'skipped' | 'unexecuted'; message: string; warning?: string }
export type BatchState = { kind: BatchKind; phase: 'idle' | 'running' | 'confirmation' | 'done'; results: BatchResult[]; conflict?: SkillUpdateResult; current: number; stopping: boolean }
type Operations = { update: (id: string, token: string) => Promise<SkillUpdateResult>; remove: (locationID: string, name: string) => Promise<string> }
const initial = (): BatchState => ({ kind: 'update', phase: 'idle', results: [], current: -1, stopping: false })

// 一个控制器只运行一条串行队列。停止只影响后续项，不中断正在写入的安装目录。
export class SkillBatchTask {
  private state = initial()
  private listeners = new Set<() => void>()
  private answer?: (decision: 'skip' | 'overwrite' | 'stop') => void
  constructor(private operations: Operations) {}
  getSnapshot = () => this.state
  subscribe = (listener: () => void) => { this.listeners.add(listener); return () => { this.listeners.delete(listener) } }
  private emit(patch: Partial<BatchState>) { this.state = { ...this.state, ...patch }; this.listeners.forEach((fn) => fn()) }
  private result(index: number, patch: Partial<BatchResult>) {
    this.emit({ results: this.state.results.map((value, i) => i === index ? { ...value, ...patch } : value) })
  }
  start(kind: BatchKind, targets: SkillInstallation[]) {
    if (this.state.phase !== 'idle' || !targets.length) return
    this.emit({ kind, phase: 'running', results: structuredClone(targets).map((item) => ({ item, status: 'pending', message: '' })), stopping: false })
    void this.run()
  }
  decide(value: 'skip' | 'overwrite' | 'stop') { const answer = this.answer; this.answer = undefined; answer?.(value) }
  stop() { if (this.state.phase === 'idle' || this.state.phase === 'done') return; this.emit({ stopping: true }); this.decide('stop') }
  close() { if (this.state.phase === 'done') { this.state = initial(); this.listeners.forEach((fn) => fn()) } }
  retry() {
    if (this.state.phase !== 'done' || !this.state.results.some((item) => item.status === 'failed')) return
    this.emit({ phase: 'running', stopping: false, results: this.state.results.map((item) => item.status === 'failed' ? { ...item, status: 'pending', message: '', warning: undefined } : item) })
    void this.run()
  }
  private async run() {
    for (let index = 0; index < this.state.results.length; index++) {
      const { item, status } = this.state.results[index]
      if (status !== 'pending') continue
      if (this.state.stopping) { this.result(index, { status: 'unexecuted', message: '任务已停止，尚未执行' }); continue }
      const reason = item.readOnly ? '系统内置，只读' : this.state.kind === 'update' ? updateUnavailable(item) : ''
      if (reason) { this.result(index, { status: 'skipped', message: reason }); continue }
      this.emit({ current: index, phase: 'running', conflict: undefined })
      this.result(index, { status: 'running' })
      try {
        if (this.state.kind === 'delete') {
          const warning = await this.operations.remove(item.locationID, item.name)
          this.result(index, { status: 'success', message: item.linked ? '已移除链接' : item.missing ? '已移除记录' : '已删除安装目录', warning })
          continue
        }
        let response = await this.operations.update(item.id, '')
        let skipped = false
        while (response.needsConfirmation) {
          if (this.state.stopping) { this.result(index, { status: 'unexecuted', message: '未覆盖本地改动' }); skipped = true; break }
          // 每个新的确认令牌都需重新征求用户意见，不能复用上一轮确认。
          const decision = await new Promise<'skip' | 'overwrite' | 'stop'>((resolve) => { this.answer = resolve; this.emit({ phase: 'confirmation', conflict: response }) })
          if (decision !== 'overwrite') {
            if (decision === 'stop') this.emit({ stopping: true })
            this.result(index, { status: decision === 'skip' ? 'skipped' : 'unexecuted', message: '保留本地改动' }); skipped = true; break
          }
          this.emit({ phase: 'running', conflict: undefined })
          response = await this.operations.update(item.id, response.confirmation)
        }
        if (!skipped) this.result(index, { status: 'success', message: response.message || '更新完成', warning: response.warning })
      } catch (error) {
        this.result(index, { status: 'failed', message: error instanceof Error ? error.message : String(error) })
      }
    }
    this.emit({ phase: 'done', current: -1, conflict: undefined })
  }
}
