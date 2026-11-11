import { createContext, useCallback, useContext, useEffect, useId, useMemo, useRef, useState, useSyncExternalStore, type Dispatch, type ReactNode, type SetStateAction } from 'react'
import { Toast } from 'radix-ui'
import { AlertCircle, CheckCircle2, X } from 'lucide-react'
import './feedback.css'

import { NoticeStore, type NoticeKind } from './noticeStore'

type Feedback = { notify: (message: string, kind?: NoticeKind, id?: string) => void; dismiss: (id: string) => void }
const Context = createContext<Feedback | null>(null)
export function FeedbackProvider({ children }: { children: ReactNode }) {
  const [store] = useState(() => new NoticeStore())
  const notices = useSyncExternalStore(store.subscribe, store.getSnapshot)
  useEffect(() => {
    const blur = () => store.pause('window'), focus = () => store.resume('window')
    if (!document.hasFocus()) blur()
    window.addEventListener('blur', blur); window.addEventListener('focus', focus)
    return () => { window.removeEventListener('blur', blur); window.removeEventListener('focus', focus); store.dispose() }
  }, [store])
  const value = useMemo(() => ({ notify: store.notify, dismiss: store.dismiss }), [store])
  return <Context.Provider value={value}><Toast.Provider swipeDirection="right">
    {children}
    {notices.map((item) => {
      const line = item.message.split('\n')[0]
      const summary = line.length > 100 ? line.slice(0, 100) + '…' : line
      return <Toast.Root key={`${item.id}-${item.revision}`} className="app-toast" data-kind={item.kind}
        duration={Infinity} onOpenChange={(open) => { if (!open) store.dismiss(item.id, item.revision) }}>
        {item.kind === 'success' ? <CheckCircle2 size={17}/> : <AlertCircle size={17}/>}
        <div><Toast.Description>{summary}</Toast.Description>{summary !== item.message && <details><summary>查看详情</summary><p>{item.message}</p></details>}</div>
        <Toast.Close className="icon-button" aria-label="关闭通知"><X size={14}/></Toast.Close>
      </Toast.Root>
    })}
    <Toast.Viewport className="app-toast-viewport" label="通知（F8）" onMouseEnter={() => store.pause('hover')} onMouseLeave={() => store.resume('hover')}
      onFocusCapture={() => store.pause('focus')} onBlurCapture={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) store.resume('focus') }} />
  </Toast.Provider></Context.Provider>
}
export function useFeedback() {
  const value = useContext(Context)
  if (!value) throw new Error('反馈组件必须位于 FeedbackProvider 中')
  return value
}
// 兼容现有保存恢复逻辑的函数式 setter，消息只占用浮层，不改变工作区布局。
export function useNoticeError(): Dispatch<SetStateAction<string>> {
  const { notify, dismiss } = useFeedback()
  const id = useId()
  const current = useRef('')
  return useCallback((value) => {
    current.current = typeof value === 'function' ? value(current.current) : value
    if (current.current) notify(current.current, 'error', id)
    else dismiss(id)
  }, [notify, dismiss, id])
}
