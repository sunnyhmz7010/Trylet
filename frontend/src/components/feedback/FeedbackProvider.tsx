import { createContext, useCallback, useContext, useId, useMemo, useRef, useState, type Dispatch, type ReactNode, type SetStateAction } from 'react'
import { Toast } from 'radix-ui'
import { AlertCircle, CheckCircle2, X } from 'lucide-react'
import './feedback.css'

type Notice = { id: string; message: string; kind: 'success' | 'error' | 'warning' }
type Feedback = { notify: (message: string, kind?: Notice['kind'], id?: string) => void; dismiss: (id: string) => void }
const Context = createContext<Feedback | null>(null)
export function FeedbackProvider({ children }: { children: ReactNode }) {
  const [notices, setNotices] = useState<Notice[]>([])
  const sequence = useRef(0)
  const dismiss = useCallback((id: string) => setNotices((items) => items.filter((item) => item.id !== id)), [])
  const notify = useCallback((message: string, kind: Notice['kind'] = 'success', id = `notice-${++sequence.current}`) => {
    if (!message.trim()) return
    setNotices((items) => [...items.filter((item) => item.id !== id), { id, message, kind }])
  }, [])
  const value = useMemo(() => ({ notify, dismiss }), [notify, dismiss])
  return <Context.Provider value={value}><Toast.Provider swipeDirection="right">
    {children}
    {notices.map((item) => <Toast.Root key={item.id} className="app-toast" data-kind={item.kind}
      duration={item.kind === 'success' ? 5000 : Infinity} onOpenChange={(open) => { if (!open) dismiss(item.id) }}>
      {item.kind === 'success' ? <CheckCircle2 size={17}/> : <AlertCircle size={17}/>}
      <Toast.Description>{item.message}</Toast.Description>
      <Toast.Close className="icon-button" aria-label="关闭通知"><X size={14}/></Toast.Close>
    </Toast.Root>)}
    <Toast.Viewport className="app-toast-viewport" label="通知（F8）" />
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
