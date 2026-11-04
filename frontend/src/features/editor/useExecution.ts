import { useCallback, useEffect, useRef, useState, type ComponentType } from 'react'
import { Events } from '@wailsio/runtime'
import * as Backend from '../../../bindings/trylet/internal/app/service'
import type { RunEvent, RuntimeCandidate } from '../../../bindings/trylet/internal/runner/models'
import type { Settings } from '../../../bindings/trylet/internal/store/models'
import { errorText, languageNames, resolvedLanguage, sessionID } from '../../lib/language'
import type { TerminalHandle } from './Terminal'
import type { DraftSession } from './useDraft'
import type { Dispatch, SetStateAction } from 'react'
type ErrorHandler = Dispatch<SetStateAction<string>>
type TerminalComponent = ComponentType<{
  onReady: (terminal: TerminalHandle) => void
  onInput: (data: string) => void
  onResize: (cols: number, rows: number) => void
}>

export function useExecution(
  drafts: DraftSession,
  settings: Settings | null,
  setError: ErrorHandler,
  setLanguageAction: (action: 'run') => void,
  openSettings: () => void,
) {
  const { draftRef, transitionLock, flush, language } = drafts
  const [runtimes, setRuntimes] = useState<RuntimeCandidate[]>([])
  const [detecting, setDetecting] = useState(false)
  const detectionGeneration = useRef(0)
  const [Terminal, setTerminal] = useState<TerminalComponent | null>(null)
  const [terminalOpen, setTerminalOpen] = useState(false)
  const [terminalHeight, setTerminalHeight] = useState(190)
  const [phase, setPhase] = useState('idle')
  const phaseRef = useRef('idle')
  const activeRun = useRef('')
  const displayedRun = useRef('')
  const terminal = useRef<TerminalHandle | null>(null)
  const terminalReady = useRef<((terminal: TerminalHandle) => void) | null>(null)
  const [runMessage, setRunMessage] = useState('')
  const updatePhase = useCallback((value: string) => {
    phaseRef.current = value
    setPhase(value)
  }, [])
  const detect = useCallback(async () => {
    const generation = ++detectionGeneration.current
    setDetecting(true)
    try {
      const found = await Backend.DetectRuntimes()
      if (generation === detectionGeneration.current) setRuntimes(found ?? [])
    } catch (failure) {
      if (generation === detectionGeneration.current) setError(errorText(failure))
    } finally {
      if (generation === detectionGeneration.current) setDetecting(false)
    }
  }, [])
  useEffect(() => {
    // 只处理当前展示会话，过滤旧进程延迟到达的事件。
    const stopRun = Events.On('trylet:run', (event) => {
      const data = event.data as RunEvent
      if (data.id !== displayedRun.current) return
      if (data.kind === 'output') {
        const bytes = Uint8Array.from(atob(data.data), (c) => c.charCodeAt(0))
        terminal.current?.write(bytes)
      } else if (data.kind === 'state') updatePhase(data.phase)
      else if (data.kind === 'exit') {
        activeRun.current = ''
        updatePhase(data.phase)
        setRunMessage(data.message)
      }
    })
    return stopRun
  }, [updatePhase])
  const available = runtimes.filter((item) => item.language === language)
  const selectedRuntime =
    available.find((item) => item.id === settings?.runtimes?.[language ?? '']) ??
    available.find((item) => item.kind === 'gcc') ??
    available[0]
  const busy = ['preparing', 'starting', 'compiling', 'running'].includes(phase)
  const onTerminalReady = useCallback((handle: TerminalHandle) => {
    terminal.current = handle
    terminalReady.current?.(handle)
    terminalReady.current = null
  }, [])
  const ensureTerminal = async () => {
    setTerminalOpen(true)
    if (terminal.current) return terminal.current
    const module = await import('./Terminal')
    const ready = new Promise<TerminalHandle>((resolve) => {
      terminalReady.current = resolve
    })
    setTerminal(() => module.default)
    return ready
  }
  const run = async () => {
    if (transitionLock.current || phaseRef.current === 'preparing') return
    if (busy) {
      if (activeRun.current) {
        try {
          await Backend.StopRun(activeRun.current)
        } catch (failure) {
          setError(errorText(failure))
        }
      }
      return
    }
    const current = draftRef.current
    if (!current) return
    const lang = resolvedLanguage(current.language, current.code)
    if (!lang) {
      setLanguageAction('run')
      return
    }
    const candidates = runtimes.filter((item) => item.language === lang)
    const runtime =
      candidates.find((item) => item.id === settings?.runtimes?.[lang]) ??
      candidates.find((item) => item.kind === 'gcc') ??
      candidates[0]
    if (!runtime) {
      setError(`未检测到可用的 ${languageNames[lang]} 环境，请在设置中指定可执行文件。`)
      openSettings()
      return
    }
    if (!current.code.trim()) {
      setError('先写几行代码，或从草稿菜单插入基础模板。')
      return
    }
    updatePhase('preparing')
    setError('')
    setRunMessage('')
    try {
      const snapshot = await flush()
      if (!snapshot) return
      const console = await ensureTerminal()
      console.clear()
      const id = sessionID()
      displayedRun.current = id
      activeRun.current = id
      await Backend.StartRun({
        id,
        draftID: snapshot.id,
        code: snapshot.code,
        language: lang,
        runtimeID: runtime.id,
        ...console.size(),
      })
      console.focus()
    } catch (failure) {
      activeRun.current = ''
      updatePhase('failed')
      setRunMessage(errorText(failure))
      setError(errorText(failure))
    }
  }
  const resizeTerminal = (event: React.PointerEvent<HTMLDivElement>) => {
    const top = event.clientY,
      initial = terminalHeight
    event.currentTarget.setPointerCapture(event.pointerId)
    const target = event.currentTarget
    const move = (e: PointerEvent) =>
      setTerminalHeight(
        Math.min(window.innerHeight - 210, Math.max(110, initial + top - e.clientY)),
      )
    const done = () => {
      target.removeEventListener('pointermove', move)
      target.removeEventListener('pointerup', done)
      target.removeEventListener('pointercancel', done)
    }
    target.addEventListener('pointermove', move)
    target.addEventListener('pointerup', done)
    target.addEventListener('pointercancel', done)
  }
  const reset = () => { setRunMessage(''); setTerminalOpen(false) }
  const clearTerminal = () => terminal.current?.clear()
  const onInput = (data: string) => {
    if (activeRun.current && phaseRef.current === 'running')
      void Backend.WriteInput(activeRun.current, data).catch((failure) => { if (activeRun.current) setError(errorText(failure)) })
  }
  const onResize = (cols: number, rows: number) => {
    if (activeRun.current) void Backend.ResizeTerminal(activeRun.current, cols, rows).catch((failure) => setError(errorText(failure)))
  }
  return {
    runtimes, detecting, detect, phase, phaseRef, activeRun, busy, selectedRuntime,
    Terminal, terminalOpen, setTerminalOpen, terminalHeight, resizeTerminal,
    onTerminalReady, onInput, onResize, runMessage, run, reset, clearTerminal,
  }
}
export type ExecutionSession = ReturnType<typeof useExecution>
