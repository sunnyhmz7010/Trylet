import { useEffect, useRef, useState, type Dispatch, type SetStateAction } from 'react'
import { Events } from '@wailsio/runtime'
import * as Desktop from '../../bindings/trylet/internal/desktop/desktop'
import type { Settings } from '../../bindings/trylet/internal/store/models'
import { errorText } from '../lib/language'

// 窗口只依赖保存和退出意图，不持有编辑器、运行或发布会话。
export function useWindowActions(
  flush: () => Promise<unknown>,
  requestQuit: () => void,
  canQuit: () => boolean,
  settings: Settings | null,
  setSettings: Dispatch<SetStateAction<Settings | null>>,
  setError: Dispatch<SetStateAction<string>>,
) {
  const [pinned, setPinned] = useState(false)
  const [largeWindow, setLargeWindow] = useState(false)
  const [closePrompt, setClosePrompt] = useState(false)
  const [rememberClose, setRememberClose] = useState(false)
  const [closeBusy, setCloseBusy] = useState(false)
  const [closeError, setCloseError] = useState('')
  const closeLock = useRef(false)
  const pinLock = useRef(false)
  const modeLock = useRef(false)
  const handlers = useRef({ hide: async () => {}, quit: () => {} })
  const hide = async () => {
    try {
      await flush()
      await Desktop.HideEditor()
    } catch (failure) {
      setError(errorText(failure))
    }
  }
  const chooseClose = async (action: 'background' | 'quit') => {
    if (closeLock.current) return
    if (action === 'quit' && !canQuit()) {
      setCloseError('当前操作尚未完成，请稍候再退出。')
      return
    }
    closeLock.current = true
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
        requestQuit()
      }
    } catch (failure) {
      setCloseError(errorText(failure))
    } finally {
      closeLock.current = false
      setCloseBusy(false)
    }
  }
  handlers.current = { hide, quit: requestQuit }
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
  const togglePinned = async () => {
    if (pinLock.current) return
    pinLock.current = true
    try { await Desktop.SetPinned(!pinned); setPinned(!pinned) }
    catch (failure) { setError(errorText(failure)) }
    finally { pinLock.current = false }
  }
  const minimise = () => { void Desktop.MinimiseEditor().catch((failure) => setError(errorText(failure))) }
  const toggleWindowMode = async () => {
    if (modeLock.current) return
    modeLock.current = true
    try { setLargeWindow(await Desktop.ToggleWindowMode()) }
    catch (failure) { setError(errorText(failure)) }
    finally { modeLock.current = false }
  }
  const closeWindow = () => {
    if (!settings) { setRememberClose(false); setCloseError(''); setClosePrompt(true); return }
    void Desktop.CloseEditor().catch((failure) => setError(errorText(failure)))
  }
  const showCloseError = (message: string) => { setCloseError(message); setClosePrompt(true) }
  return { pinned, largeWindow, closePrompt, setClosePrompt, rememberClose, setRememberClose,
    closeBusy, closeError, chooseClose, togglePinned, minimise, toggleWindowMode, closeWindow, hide, showCloseError }
}
export type WindowActions = ReturnType<typeof useWindowActions>
