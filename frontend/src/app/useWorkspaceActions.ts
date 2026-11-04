import { useEffect, useRef, useState } from 'react'
import { Events } from '@wailsio/runtime'
import * as Backend from '../../bindings/trylet/internal/app/service'
import * as Desktop from '../../bindings/trylet/internal/desktop/desktop'
import type { Settings } from '../../bindings/trylet/internal/store/models'
import { errorText } from '../lib/language'
import type { DraftSession } from '../features/editor/useDraft'
import type { ExecutionSession } from '../features/editor/useExecution'
import type { PublishingSession } from '../features/publish/usePublishing'
import type { Dispatch, SetStateAction } from 'react'
type ErrorHandler = Dispatch<SetStateAction<string>>
export type View = 'editor' | 'git' | 'skills' | 'settings'
type PendingAction = { kind: 'new' | 'load' | 'quit'; id?: string }

export function useWorkspaceActions(
  drafts: DraftSession,
  execution: ExecutionSession,
  publishing: PublishingSession,
  settings: Settings | null,
  setSettings: Dispatch<SetStateAction<Settings | null>>,
  setError: ErrorHandler,
  activeView: View,
) {
  const { transitionLock, flush, hydrateDraft, refreshHistory } = drafts
  const { phaseRef, activeRun, run } = execution
  const { publish } = publishing
  const activeViewRef = useRef(activeView)
  activeViewRef.current = activeView
  const [pending, setPending] = useState<PendingAction | null>(null)
  const [switching, setSwitching] = useState(false)
  const [pinned, setPinned] = useState(false)
  const [largeWindow, setLargeWindow] = useState(false)
  const [closePrompt, setClosePrompt] = useState(false)
  const [rememberClose, setRememberClose] = useState(false)
  const [closeBusy, setCloseBusy] = useState(false)
  const [closeError, setCloseError] = useState('')
  // 窗口事件和快捷键读取最新操作，避免持有旧草稿或会话。
  const handlers = useRef({ hide: async () => {}, quit: () => {}, run: async () => {}, create: () => {}, publish: async () => {} })
  const executeAction = async (action: PendingAction) => {
    if (transitionLock.current || phaseRef.current === 'preparing') return
    transitionLock.current = true
    setSwitching(true)
    try {
      await flush()
      if (activeRun.current) await Backend.StopRun(activeRun.current)
      if (action.kind === 'quit') {
        await Desktop.Quit()
        return
      }
      const value =
        action.kind === 'new' ? await Backend.NewDraft() : await Backend.LoadDraft(action.id!)
      hydrateDraft(value)
      execution.reset()
      await refreshHistory()
      setError('')
    } catch (failure) {
      setError(errorText(failure))
      if (action.kind === 'quit') {
        setCloseError(errorText(failure))
        setClosePrompt(true)
      }
    } finally {
      transitionLock.current = false
      setSwitching(false)
      setPending(null)
    }
  }
  const requestAction = (action: PendingAction) => {
    if (transitionLock.current) return
    if (phaseRef.current === 'preparing') {
      setError('正在准备运行，请稍候再切换练习。')
      return
    }
    if (activeRun.current) setPending(action)
    else void executeAction(action)
  }
  const hide = async () => {
    try {
      await flush()
      await Desktop.HideEditor()
    } catch (failure) {
      setError(errorText(failure))
    }
  }
  const chooseClose = async (action: 'background' | 'quit') => {
    if (closeBusy) return
    if (action === 'quit' && phaseRef.current === 'preparing') {
      setCloseError('正在准备运行，请稍候再退出。')
      return
    }
    setCloseBusy(true)
    setCloseError('')
    try {
      if (rememberClose) setSettings(await Desktop.RememberCloseAction(action))
      if (action === 'background') {
        await flush()
        await Desktop.HideEditor()
        setClosePrompt(false)
      } else {
        setClosePrompt(false)
        requestAction({ kind: 'quit' })
      }
    } catch (failure) {
      setCloseError(errorText(failure))
    } finally {
      setCloseBusy(false)
    }
  }
  handlers.current = {
    hide,
    quit: () => requestAction({ kind: 'quit' }),
    run,
    create: () => requestAction({ kind: 'new' }),
    publish: async () => publish(),
  }
  useEffect(() => {
    const hide = Events.On('trylet:hide-requested', () => {
      void handlers.current.hide()
    })
    const quit = Events.On('trylet:quit-requested', () => handlers.current.quit())
    const close = Events.On('trylet:close-requested', () => {
      if (document.querySelector('[role="dialog"]')) return
      setRememberClose(false)
      setCloseError('')
      setClosePrompt(true)
    })
    const mode = Events.On('trylet:window-mode', (event) => setLargeWindow(event.data))
    const preferences = Events.On('trylet:settings', (event) => setSettings(event.data))
    return () => { hide(); quit(); close(); mode(); preferences() }
  }, [])
  useEffect(() => {
    const key = (event: KeyboardEvent) => {
      if (event.isComposing || document.querySelector('[role="dialog"]')) return
      if (activeViewRef.current !== 'editor') return
      if (event.ctrlKey && event.key === 'Enter') {
        event.preventDefault()
        void handlers.current.run()
      } else if (event.ctrlKey && !event.shiftKey && event.key.toLowerCase() === 'n') {
        event.preventDefault()
        handlers.current.create()
      } else if (event.ctrlKey && event.shiftKey && event.key.toLowerCase() === 's') {
        event.preventDefault()
        void handlers.current.publish()
      } else if (event.ctrlKey && event.key.toLowerCase() === 's') {
        event.preventDefault()
        void flush().catch((failure) => setError(errorText(failure)))
      } else if (
        event.key === 'Escape' &&
        !document.querySelector('[data-state="open"][role="menu"]') &&
        !(event.target as HTMLElement).closest('.terminal-mount')
      ) {
        event.preventDefault()
        void handlers.current.hide()
      }
    }
    window.addEventListener('keydown', key)
    return () => window.removeEventListener('keydown', key)
  }, [flush])
  const togglePinned = () => { void Desktop.SetPinned(!pinned).then(() => setPinned(!pinned)).catch((failure) => setError(errorText(failure))) }
  const minimise = () => { void Desktop.MinimiseEditor().catch((failure) => setError(errorText(failure))) }
  const toggleWindowMode = () => { void Desktop.ToggleWindowMode().then(setLargeWindow).catch((failure) => setError(errorText(failure))) }
  const closeWindow = () => {
    if (!settings) { setRememberClose(false); setCloseError(''); setClosePrompt(true); return }
    void Desktop.CloseEditor().catch((failure) => setError(errorText(failure)))
  }
  return {
    pending, setPending, switching, pinned, largeWindow,
    closePrompt, setClosePrompt, rememberClose, setRememberClose, closeBusy, closeError,
    requestAction, executeAction, chooseClose, togglePinned, minimise, toggleWindowMode, closeWindow,
  }
}
export type WorkspaceActions = ReturnType<typeof useWorkspaceActions>
