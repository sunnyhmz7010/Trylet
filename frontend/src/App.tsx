import { useCallback, useEffect, useRef, useState, type ComponentType } from 'react'
import { Events } from '@wailsio/runtime'
import appLogo from '../../build/icon.svg'
import {
  AlertCircle,
  ChevronDown,
  Code2,
  FolderClock,
  Github,
  GitBranch,
  LoaderCircle,
  Minus,
  Pin,
  Play,
  Plus,
  Puzzle,
  Settings2,
  Square,
  TerminalSquare,
  Trash2,
  X,
} from 'lucide-react'
import * as Backend from '../bindings/trylet/internal/trylet/service'
import * as Desktop from '../bindings/trylet/desktop'
import type {
  Draft,
  DraftSummary,
  PublishRecord,
  RunEvent,
  RuntimeCandidate,
  Settings,
} from '../bindings/trylet/internal/trylet/models'
import { Button } from './components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from './components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from './components/ui/dropdown-menu'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from './components/ui/select'
import { Input } from './components/ui/input'
import Editor from './Editor'
import SettingsPanel from './SettingsPanel'
import { SettingsContext } from './SettingsContext'
import { watchAppearance } from './appearance'
import PublishDialog from './PublishDialog'
import type { TerminalHandle } from './Terminal'
import {
  errorText,
  languageNames,
  resolvedLanguage,
  sessionID,
  templates,
  type Language,
} from './lib/language'

type TerminalComponent = ComponentType<{
  onReady: (terminal: TerminalHandle) => void
  onInput: (data: string) => void
  onResize: (cols: number, rows: number) => void
}>
type PendingAction = { kind: 'new' | 'load' | 'quit'; id?: string }

export default function App() {
  const [draft, setDraft] = useState<Draft | null>(null)
  const draftRef = useRef<Draft | null>(null)
  const [settings, setSettings] = useState<Settings | null>(null)
  useEffect(() => watchAppearance(settings?.theme ?? 'system'), [settings?.theme])
  const [tokenSet, setTokenSet] = useState(false)
  const [history, setHistory] = useState<DraftSummary[]>([])
  const [runtimes, setRuntimes] = useState<RuntimeCandidate[]>([])
  const [detecting, setDetecting] = useState(false)
  const [error, setError] = useState('')
  const [bootError, setBootError] = useState('')
  const [saveStatus, setSaveStatus] = useState('已保存')
  const [activeView, setActiveView] = useState<'editor' | 'git' | 'skills' | 'settings'>('editor')
  const activeViewRef = useRef(activeView)
  activeViewRef.current = activeView
  const [GitView, setGitView] = useState<ComponentType<{active:boolean}> | null>(null)
  const [gitLoadError, setGitLoadError] = useState('')
  useEffect(() => {
    if (activeView !== 'git' || GitView || gitLoadError) return
    void import('./GitPanel').then((module) => setGitView(() => module.default)).catch((failure) => setGitLoadError(errorText(failure)))
  },[activeView,GitView,gitLoadError])
  const [SkillsView, setSkillsView] = useState<ComponentType<{active:boolean}> | null>(null)
  const [skillsLoadError, setSkillsLoadError] = useState('')
  useEffect(() => {
    if (activeView !== 'skills' || SkillsView || skillsLoadError) return
    void import('./SkillsPanel').then((module) => setSkillsView(() => module.default)).catch((failure) => setSkillsLoadError(errorText(failure)))
  }, [activeView, SkillsView, skillsLoadError])
  const [publishDraft, setPublishDraft] = useState<Draft | null>(null)
  const [pending, setPending] = useState<PendingAction | null>(null)
  const [languageAction, setLanguageAction] = useState<'run' | 'publish' | 'template' | null>(null)
  const [manualLanguage, setManualLanguage] = useState<Language>('python')
  const [rename, setRename] = useState<string | null>(null)
  const renameOwner = useRef('')
  const [switching, setSwitching] = useState(false)
  const transitionLock = useRef(false)
  const detectionGeneration = useRef(0)
  const [pinned, setPinned] = useState(false)
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
  const version = useRef(0),
    savedVersion = useRef(0),
    queuedVersion = useRef(0)
  const saveChain = useRef<Promise<Draft | undefined>>(Promise.resolve(undefined))
  const publishAfterSettings = useRef(false)
  const handlers = useRef({
    hide: async () => {},
    quit: () => {},
    run: async () => {},
    create: () => {},
    publish: async () => {},
  })

  const updatePhase = useCallback((value: string) => {
    phaseRef.current = value
    setPhase(value)
  }, [])
  const updateDraft = useCallback((patch: Partial<Draft>) => {
    if (!draftRef.current || transitionLock.current) return false
    const next = { ...draftRef.current, ...patch }
    draftRef.current = next
    version.current++
    setDraft(next)
    setSaveStatus('待保存')
    return true
  }, [])
  const flush = useCallback(async () => {
    const snapshot = draftRef.current
    if (!snapshot) return undefined
    const revision = version.current
    if (revision <= queuedVersion.current) {
      await saveChain.current
      return snapshot
    }
    queuedVersion.current = revision
    const operation = async () => {
      setSaveStatus('保存中')
      try {
        const result = await Backend.SaveDraft({
          id: snapshot.id,
          title: snapshot.title,
          language: snapshot.language,
          code: snapshot.code,
        })
        if (draftRef.current?.id === snapshot.id) {
          savedVersion.current = Math.max(savedVersion.current, revision)
          setSaveStatus(savedVersion.current === version.current ? '已保存' : '待保存')
          setHistory((items) => [
            {
              id: result.id,
              title: result.title,
              language: result.language,
              updatedAt: result.updatedAt,
            },
            ...items.filter((item) => item.id !== result.id),
          ])
        }
        return result
      } catch (failure) {
        if (queuedVersion.current === revision) queuedVersion.current = savedVersion.current
        setSaveStatus('保存失败')
        setError(errorText(failure))
        throw failure
      }
    }
    saveChain.current = saveChain.current.then(operation, operation)
    await saveChain.current
    return snapshot
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

  const boot = useCallback(async () => {
    setBootError('')
    try {
      const data = await Backend.Bootstrap()
      draftRef.current = data.draft
      setDraft(data.draft)
      setSettings(data.settings)
      setTokenSet(data.tokenSet)
      setHistory(data.history ?? [])
      if (data.notice) setError(data.notice)
      const notice = await Desktop.EditorReady()
      if (notice) setError(notice)
      void detect()
    } catch (failure) {
      setBootError(errorText(failure))
    }
  }, [detect])

  useEffect(() => {
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
    const hide = Events.On('trylet:hide-requested', () => {
      void handlers.current.hide()
    })
    const quit = Events.On('trylet:quit-requested', () => handlers.current.quit())
    void boot()
    return () => {
      stopRun()
      hide()
      quit()
    }
  }, [boot, updatePhase])

  useEffect(() => {
    if (!draft || version.current === savedVersion.current) return
    const timer = window.setTimeout(() => {
      void flush().catch(() => {
        /* 错误已由保存入口显示，保留待保存内容。 */
      })
    }, 400)
    return () => clearTimeout(timer)
  }, [draft?.code, draft?.language, draft?.title, draft?.id, flush])

  const language = draft ? resolvedLanguage(draft.language, draft.code) : null
  const available = runtimes.filter((item) => item.language === language)
  const selectedRuntime =
    available.find((item) => item.id === settings?.runtimes?.[language ?? '']) ??
    available.find((item) => item.kind === 'gcc') ??
    available[0]
  const busy = ['preparing', 'starting', 'compiling', 'running'].includes(phase)

  const hydrateDraft = (value: Draft) => {
    draftRef.current = value
    version.current = 0
    queuedVersion.current = 0
    savedVersion.current = 0
    setDraft(value)
    setSaveStatus('已保存')
    setRunMessage('')
    setTerminalOpen(false)
  }
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
      setHistory((await Backend.History()) ?? [])
      setError('')
    } catch (failure) {
      setError(errorText(failure))
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
      setActiveView('settings')
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
  const publish = async (newSettings = settings, hasToken = tokenSet) => {
    if (transitionLock.current) return
    const current = draftRef.current
    if (!current) return
    const lang = resolvedLanguage(current.language, current.code)
    if (!lang) {
      setLanguageAction('publish')
      return
    }
    if (!current.code.trim()) {
      setError('空白草稿不能发布，请先编写代码。')
      return
    }
    if (!newSettings?.repository || !hasToken) {
      publishAfterSettings.current = true
      setActiveView('settings')
      return
    }
    try {
      const snapshot = await flush()
      if (snapshot) setPublishDraft({ ...snapshot, language: lang })
    } catch (failure) {
      setError(errorText(failure))
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

  const changeLanguage = (value: string) => {
    const current = draftRef.current
    if (!current) return
    const patch: Partial<Draft> = { language: value }
    if (!current.code.trim() && settings?.autoTemplate && value !== 'auto')
      patch.code = templates[value as Language]
    updateDraft(patch)
  }
  const insertTemplate = () => {
    if (draftRef.current?.code.trim()) {
      setError('基础模板只填入空白草稿，请先新建练习。')
      return
    }
    if (!language) {
      setLanguageAction('template')
      return
    }
    updateDraft({ code: templates[language] })
  }
  const published = (record: PublishRecord) => {
    const current = draftRef.current
    if (!current || current.id !== publishDraft?.id) return
    const title =
      current.title === '未命名练习' ? record.target.path.split('/').at(-1)! : current.title
    draftRef.current = { ...current, title, publish: record }
    setDraft(draftRef.current)
    void Backend.History()
      .then((v) => setHistory(v ?? []))
      .catch((failure) => setError(errorText(failure)))
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

  if (!draft || !settings)
    return (
      <div className="boot-screen">
        <img src={appLogo} width={32} height={32} alt="" />
        <span>Trylet</span>
        {bootError ? (
          <>
            <p>{bootError}</p>
            <Button variant="outline" onClick={() => void boot()}>
              重试加载
            </Button>
          </>
        ) : (
          <p>正在恢复你的草稿…</p>
        )}
      </div>
    )
  return (
    <SettingsContext.Provider value={settings}><div className="app-shell">
      <header className="app-navigation">
        <div className="app-tabs" role="tablist" aria-label="工作区" onKeyDown={(event) => {
          const views = ['editor','git','skills','settings'] as const
          const index = views.indexOf(activeView)
          const next = event.key === 'ArrowRight' ? (index+1)%views.length : event.key === 'ArrowLeft' ? (index+views.length-1)%views.length : event.key === 'Home' ? 0 : event.key === 'End' ? views.length-1 : -1
          if (next < 0) return
          event.preventDefault()
          publishAfterSettings.current = false
          setActiveView(views[next])
          ;(event.currentTarget.children[next] as HTMLButtonElement).focus()
        }}>
          {([{view:'editor',label:'编辑器',Icon:Code2},{view:'git',label:'Git',Icon:GitBranch},{view:'skills',label:'Skills',Icon:Puzzle},{view:'settings',label:'设置',Icon:Settings2}] as const).map(({view,label,Icon}) => (
            <button key={view} id={'tab-'+view} className="folder-tab" role="tab" aria-label={label} title={label} aria-selected={activeView===view} aria-controls={'panel-'+view} tabIndex={activeView===view?0:-1} onClick={() => {publishAfterSettings.current=false;setActiveView(view)}}>
              <Icon size={17} aria-hidden="true"/>
            </button>
          ))}
        </div>
        <div className="top-spacer" />
        <button
          className={`icon-button ${pinned ? 'active' : ''}`}
          aria-label={pinned ? '取消置顶' : '置顶窗口'}
          title={pinned ? '取消置顶' : '置顶窗口'}
          onClick={() => {
            void Desktop.SetPinned(!pinned)
              .then(() => setPinned(!pinned))
              .catch((failure) => setError(errorText(failure)))
          }}
        >
          <Pin size={15} />
        </button>
        <button
          className="icon-button"
          aria-label="收起窗口"
          title="收起窗口"
          onClick={() => void hide()}
        >
          <Minus size={17} />
        </button>

      </header>
      <section id="panel-editor" role="tabpanel" aria-labelledby="tab-editor" className="editor-page" hidden={activeView!=='editor'}>
      <header className="page-toolbar">
        <Select disabled={switching} value={draft.language} onValueChange={changeLanguage}>
          <SelectTrigger className="language-trigger" aria-label="选择语言">
            <SelectValue>
              {draft.language === 'auto'
                ? `自动${language ? ` · ${languageNames[language]}` : ''}`
                : languageNames[draft.language]}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {Object.entries(languageNames).map(([value, label]) => (
              <SelectItem key={value} value={value}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <DropdownMenu
          onOpenChange={(open) => {
            if (open)
              void Backend.History()
                .then((v) => setHistory(v ?? []))
                .catch((failure) => setError(errorText(failure)))
          }}
        >
          <DropdownMenuTrigger
            disabled={switching}
            className="draft-trigger"
            title="草稿与近期练习"
          >
            <span>{draft.title}</span>
            <ChevronDown size={12} />
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="max-h-[320px] w-[310px] overflow-y-auto">
            <DropdownMenuItem disabled={switching} onSelect={() => requestAction({ kind: 'new' })}>
              <Plus size={14} />
              新建练习<span className="history-date">Ctrl+N</span>
            </DropdownMenuItem>
            <DropdownMenuItem disabled={switching} onSelect={insertTemplate}>
              <Code2 size={14} />
              插入基础模板
            </DropdownMenuItem>
            <DropdownMenuItem
              disabled={switching}
              onSelect={() => {
                renameOwner.current = draft.id
                setRename(draft.title)
              }}
            >
              重命名练习
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            {history.map((item) => (
              <DropdownMenuItem
                key={item.id}
                disabled={item.id === draft.id || switching}
                onSelect={() => requestAction({ kind: 'load', id: item.id })}
              >
                <div className="history-item">
                  <FolderClock size={13} />
                  <span className="history-name">{item.title}</span>
                  <span className="history-date">
                    {languageNames[item.language]} ·{' '}
                    {new Date(item.updatedAt).toLocaleTimeString('zh-CN', {
                      hour: '2-digit',
                      minute: '2-digit',
                    })}
                  </span>
                </div>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>

      </header>
      <main className="workspace">
        {error && (
          <div className="notice" role="alert">
            <AlertCircle size={14} />
            <span>{error}</span>
            <button className="icon-button" aria-label="关闭提示" onClick={() => setError('')}>
              <X size={13} />
            </button>
          </div>
        )}
        <div className="editor-holder">
          <Editor
            key={draft.id}
            value={draft.code}
            language={language}
            locked={switching}
            onChange={(value) => updateDraft({ code: value })}
          />
          {!draft.code && <div className="editor-hint">选择语言可填入模板 · Ctrl+Enter 运行</div>}
        </div>
        {Terminal && (
          <section
            className="terminal-panel"
            aria-label="运行终端"
            style={{ height: terminalHeight, display: terminalOpen ? 'flex' : 'none' }}
          >
            <div
              className="resize-handle"
              role="separator"
              aria-label="调整终端高度"
              onPointerDown={resizeTerminal}
            />
            <div className="terminal-header">
              <TerminalSquare size={13} />
              <strong>终端</strong>
              <span className={`terminal-result ${phase === 'failed' ? 'failed' : ''}`}>
                {busy
                  ? {
                      preparing: '准备运行…',
                      starting: '启动中…',
                      compiling: '正在编译…',
                      running: '运行中',
                    }[phase]
                  : runMessage}
              </span>
              <button
                className="icon-button"
                aria-label="清空终端"
                title="清空终端"
                onClick={() => terminal.current?.clear()}
              >
                <Trash2 size={12} />
              </button>
              <button
                className="icon-button"
                aria-label="收起终端"
                title="收起终端"
                onClick={() => setTerminalOpen(false)}
              >
                <Minus size={14} />
              </button>
            </div>
            <Terminal
              onReady={onTerminalReady}
              onInput={(data) => {
                if (activeRun.current && phaseRef.current === 'running')
                  void Backend.WriteInput(activeRun.current, data).catch((failure) => {
                    if (activeRun.current) setError(errorText(failure))
                  })
              }}
              onResize={(cols, rows) => {
                if (activeRun.current)
                  void Backend.ResizeTerminal(activeRun.current, cols, rows).catch((failure) =>
                    setError(errorText(failure)),
                  )
              }}
            />
          </section>
        )}
      </main>
      <footer className="footer page-footer">
        <button
          className="environment"
          title={
            selectedRuntime ? `${selectedRuntime.name} · ${selectedRuntime.path}` : '选择本机环境'
          }
          onClick={() => setActiveView('settings')}
        >
          {detecting ? (
            <LoaderCircle size={11} className="animate-spin" />
          ) : (
            <i className={`status-dot ${selectedRuntime ? 'ready' : ''}`} />
          )}
          <span>
            {detecting
              ? '检测本机环境…'
              : selectedRuntime
                ? `${selectedRuntime.name} ${selectedRuntime.kind === 'python' ? selectedRuntime.version : ''}`
                : language
                  ? '选择运行环境'
                  : '自动识别语言'}
          </span>
        </button>
        <span className="footer-status">{saveStatus}</span>
        {Terminal && !terminalOpen && (
          <button
            className="icon-button"
            aria-label="展开终端"
            title="展开终端"
            onClick={() => setTerminalOpen(true)}
          >
            <TerminalSquare size={14} />
          </button>
        )}
        <div className="footer-actions">
          <Button
            className="action-button"
            variant="outline"
            onClick={() => void publish()}
            disabled={switching}
          >
            <Github size={13} />
            发布
          </Button>
          <Button
            className="action-button"
            variant={busy ? 'destructive' : 'default'}
            onClick={() => void run()}
            disabled={switching || phase === 'preparing'}
          >
            {busy ? (
              <Square size={11} fill="currentColor" />
            ) : (
              <Play size={12} fill="currentColor" />
            )}
            {busy ? '停止' : '运行'}
            {!busy && <span className="shortcut">Ctrl ↵</span>}
          </Button>
        </div>
      </footer>
      </section>
      <section id="panel-git" role="tabpanel" aria-labelledby="tab-git" className="git-page" hidden={activeView!=='git'}>
        {GitView ? <GitView active={activeView==='git'}/> : <div className="panel-loading">{gitLoadError ? <><span>{gitLoadError}</span><Button variant="outline" onClick={()=>setGitLoadError('')}>重试</Button></> : <><LoaderCircle size={18} className="animate-spin"/>正在加载 Git 视图…</>}</div>}
      </section>
      <section id="panel-skills" role="tabpanel" aria-labelledby="tab-skills" className="skills-page" hidden={activeView!=='skills'}>
        {SkillsView ? <SkillsView active={activeView==='skills'}/> : <div className="panel-loading">{skillsLoadError ? <><span>{skillsLoadError}</span><Button variant="outline" onClick={()=>setSkillsLoadError('')}>重试</Button></> : <><LoaderCircle size={18} className="animate-spin"/>正在加载 Skills…</>}</div>}
      </section>
      <section id="panel-settings" role="tabpanel" aria-labelledby="tab-settings" className="settings-page" hidden={activeView!=='settings'}>
        <SettingsPanel
          active={activeView==='settings'}
          settings={settings}
          tokenSet={tokenSet}
          runtimes={runtimes}
          detecting={detecting}
          onDetect={detect}
          onClose={() => {
            publishAfterSettings.current = false
            setActiveView('editor')
          }}
          onTokenRemoved={() => setTokenSet(false)}
          onSaved={(value, hasToken) => {
            setSettings(value)
            setTokenSet(hasToken)
            void detect()
            if (publishAfterSettings.current && activeViewRef.current==='settings') {
              publishAfterSettings.current = false
              setActiveView('editor')
              void publish(value, hasToken)
            }
          }}
        />
      </section>
      {publishDraft && (
        <PublishDialog
          draft={publishDraft}
          settings={settings}
          onClose={() => setPublishDraft(null)}
          onPublished={published}
        />
      )}
      <Dialog
        open={pending !== null}
        onOpenChange={(open) => {
          if (!open && !switching) setPending(null)
        }}
      >
        <DialogContent className="dialog-content" aria-describedby="stop-description">
          <div className="dialog-heading">
            <DialogTitle>停止当前运行？</DialogTitle>
            <DialogDescription id="stop-description">
              当前练习仍在运行。停止后继续{pending?.kind === 'quit' ? '退出应用' : '切换练习'}
              ，代码会先保存。
            </DialogDescription>
          </div>
          <div className="dialog-footer">
            <Button variant="outline" disabled={switching} onClick={() => setPending(null)}>
              取消
            </Button>
            <Button
              disabled={switching}
              onClick={() => {
                if (pending) void executeAction(pending)
              }}
            >
              {switching ? '正在停止…' : '停止并继续'}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
      <Dialog
        open={languageAction !== null}
        onOpenChange={(open) => {
          if (!open) setLanguageAction(null)
        }}
      >
        <DialogContent className="dialog-content">
          <div className="dialog-heading">
            <DialogTitle>选择这份练习的语言</DialogTitle>
            <DialogDescription>
              当前代码还不能明确识别。C 和 C++ 可能使用相同语法，请确认后继续。
            </DialogDescription>
          </div>
          <div className="dialog-scroll">
            <select
              aria-label="练习语言"
              className="native-select"
              value={manualLanguage}
              onChange={(e) => setManualLanguage(e.target.value as Language)}
            >
              {Object.keys(templates).map((lang) => (
                <option key={lang} value={lang}>
                  {languageNames[lang]}
                </option>
              ))}
            </select>
          </div>
          <div className="dialog-footer">
            <Button variant="outline" onClick={() => setLanguageAction(null)}>
              取消
            </Button>
            <Button
              onClick={() => {
                const action = languageAction
                changeLanguage(manualLanguage)
                setLanguageAction(null)
                if (action === 'run') void run()
                else if (action === 'publish') void publish()
                else updateDraft({ code: templates[manualLanguage] })
              }}
            >
              确认语言
            </Button>
          </div>
        </DialogContent>
      </Dialog>
      <Dialog
        open={rename !== null}
        onOpenChange={(open) => {
          if (!open) setRename(null)
        }}
      >
        <DialogContent className="dialog-content">
          <div className="dialog-heading">
            <DialogTitle>重命名练习</DialogTitle>
            <DialogDescription>这个名称用于在近期历史中找到练习。</DialogDescription>
          </div>
          <div className="dialog-scroll">
            <Input
              autoFocus
              aria-label="练习名称"
              value={rename ?? ''}
              onChange={(e) => setRename(e.target.value)}
              maxLength={100}
            />
          </div>
          <div className="dialog-footer">
            <Button variant="outline" onClick={() => setRename(null)}>
              取消
            </Button>
            <Button
              disabled={switching || !rename?.trim()}
              onClick={() => {
                if (draftRef.current?.id !== renameOwner.current) {
                  setError('练习已切换，请取消重命名后重试。')
                  return
                }
                if (updateDraft({ title: rename!.trim() })) setRename(null)
              }}
            >
              保存名称
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </div></SettingsContext.Provider>
  )
}
