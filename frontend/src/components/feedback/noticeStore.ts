export type NoticeKind = 'success' | 'warning' | 'error'
export type Notice = { id: string; revision: number; message: string; kind: NoticeKind }
export const noticeDuration: Record<NoticeKind, number> = { success: 3000, warning: 5000, error: 8000 }
export class NoticeStore {
  private items: Notice[] = []
  private listeners = new Set<() => void>()
  private paused = new Set<string>()
  private timers = new Map<string, { remaining: number; started: number; timer?: ReturnType<typeof setTimeout> }>()
  private sequence = 0
  getSnapshot = () => this.items
  subscribe = (fn: () => void) => { this.listeners.add(fn); return () => { this.listeners.delete(fn) } }
  private emit() { this.listeners.forEach((fn) => fn()) }
  notify = (message: string, kind: NoticeKind = 'success', requestedID?: string) => {
    message = message.trim()
    if (!message) return
    const existing = this.items.find((item) => item.id === requestedID || (item.message === message && item.kind === kind))
    const id = existing?.id ?? requestedID ?? `notice-${++this.sequence}`
    this.clearTimer(id)
    const notice = { id, revision: ++this.sequence, message, kind }
    this.items = [...this.items.filter((item) => item.id !== id), notice]
    while (this.items.length > 3) { const removed = this.items.shift()!; this.clearTimer(removed.id) }
    this.timers.set(id, { remaining: noticeDuration[kind], started: Date.now() })
    this.schedule(id)
    this.emit()
  }
  dismiss = (id: string, revision?: number) => {
    if (revision !== undefined && !this.items.some((item) => item.id === id && item.revision === revision)) return
    this.clearTimer(id); this.items = this.items.filter((item) => item.id !== id); this.emit()
  }
  pause = (reason: string) => {
    if (this.paused.has(reason)) return
    if (!this.paused.size) for (const entry of this.timers.values()) {
      clearTimeout(entry.timer); entry.timer = undefined
      entry.remaining = Math.max(0, entry.remaining - (Date.now() - entry.started))
    }
    this.paused.add(reason)
  }
  resume = (reason: string) => {
    if (!this.paused.delete(reason) || this.paused.size) return
    for (const id of this.timers.keys()) this.schedule(id)
  }
  private schedule(id: string) {
    const entry = this.timers.get(id)
    if (!entry || this.paused.size) return
    entry.started = Date.now()
    entry.timer = setTimeout(() => this.dismiss(id), entry.remaining)
  }
  private clearTimer(id: string) { clearTimeout(this.timers.get(id)?.timer); this.timers.delete(id) }
  dispose() { for (const id of this.timers.keys()) this.clearTimer(id) }
}
