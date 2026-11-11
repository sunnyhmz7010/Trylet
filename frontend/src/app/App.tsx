import { useCallback, useEffect, useState } from 'react'
import { Code2, GitBranch, LoaderCircle, Pin, Puzzle, Settings2 } from 'lucide-react'
import appLogo from '../../../build/icon.svg'
import * as Backend from '../../bindings/trylet/internal/app/service'
import * as Desktop from '../../bindings/trylet/internal/desktop/desktop'
import type { Settings } from '../../bindings/trylet/internal/store/models'
import { Button } from '../components/ui/button'
import { SettingsContext } from '../features/settings/SettingsContext'
import { watchAppearance } from '../appearance'
import { errorText } from '../lib/language'
import { useDraft } from '../features/editor/useDraft'
import { useEditorCommands } from '../features/editor/useEditorCommands'
import { useExecution } from '../features/editor/useExecution'
import { usePublishing } from '../features/publish/usePublishing'
import EditorWorkspace from '../features/editor/EditorWorkspace'
import DraftDialogs from '../features/editor/DraftDialogs'
import PublishOverlay from '../features/publish/PublishOverlay'
import { useWorkspaceActions, type View } from './useWorkspaceActions'
import { CloseDialog, PendingDialog } from './WorkspaceDialogs'
import WindowControls from './WindowControls'
import { useNoticeError } from '../components/feedback/FeedbackProvider'
import { useDeferredModule } from '../lib/useDeferredModule'

const loadSettings = () => import('../features/settings/SettingsPanel')
const loadGit = () => import('../features/git/GitPanel')
const loadSkills = () => import('../features/skills/SkillsPanel')

export default function App() {
  const [settings, setSettings] = useState<Settings | null>(null)
  useEffect(() => watchAppearance(settings?.theme ?? 'system'), [settings?.theme])
  const [tokenSet, setTokenSet] = useState(false)
  const setError = useNoticeError()
  const [bootError, setBootError] = useState('')
  const [activeView, setActiveView] = useState<View>('editor')
  const [settingsCategory, setSettingsCategory] = useState('general')
  const [Editor, setEditor] = useState<typeof import('../features/editor/Editor').default | null>(null)
  const { component: SettingsView, error: settingsLoadError, retry: retrySettings } = useDeferredModule(activeView === 'settings', loadSettings)
  const { component: GitView, error: gitLoadError, retry: retryGit } = useDeferredModule(activeView === 'git', loadGit)
  const { component: SkillsView, error: skillsLoadError, retry: retrySkills } = useDeferredModule(activeView === 'skills', loadSkills)
  const drafts = useDraft(setError)
  const commands = useEditorCommands(drafts, settings, setError)
  const openSettings = (category: string) => { setSettingsCategory(category); setActiveView('settings') }
  const execution = useExecution(drafts, settings, setError, commands.setLanguageAction, () => openSettings('runtimes'))
  const publishing = usePublishing(drafts, settings, tokenSet, setError, commands.setLanguageAction, () => openSettings('github'))
  const actions = useWorkspaceActions(drafts, execution, publishing, settings, setSettings, setError, activeView)
  const { draft, restore } = drafts
  const { runtimes, detecting, detect } = execution
  const { cancelContinuation, resumeAfterSettings, publish } = publishing
  const boot = useCallback(async () => {
    setBootError('')
    try {
      // 草稿读取与编辑器模块并行，避免语法和编辑器代码阻塞本机数据请求。
      const [data, editor] = await Promise.all([Backend.Bootstrap(), import('../features/editor/Editor')])
      setEditor(() => editor.default)
      restore(data.draft, data.history ?? [])
      setSettings(data.settings)
      setTokenSet(data.tokenSet)
      if (data.notice) setError(data.notice)
      const notice = await Desktop.EditorReady()
      if (notice) setError(notice)
      void detect()
    } catch (failure) {
      setBootError(errorText(failure))
    }
  }, [detect, restore])
  useEffect(() => { void boot() }, [boot])
  const windowControls = <WindowControls actions={actions} />
  const closeDialog = <CloseDialog actions={actions} />
  if (!draft || !settings || !Editor)
    return (
      <div className="app-shell">
      <header className="app-navigation"><div className="top-spacer" />{windowControls}</header>
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
      {closeDialog}
      </div>
    )
  return <SettingsContext.Provider value={settings}><div className="app-shell">
      <header className="app-navigation">
        <div className="app-tabs" role="tablist" aria-label="工作区" onKeyDown={(event) => {
          const views = ['editor','git','skills','settings'] as const
          const index = views.indexOf(activeView)
          const next = event.key === 'ArrowRight' ? (index+1)%views.length : event.key === 'ArrowLeft' ? (index+views.length-1)%views.length : event.key === 'Home' ? 0 : event.key === 'End' ? views.length-1 : -1
          if (next < 0) return
          event.preventDefault()
          cancelContinuation()
          setActiveView(views[next])
          ;(event.currentTarget.children[next] as HTMLButtonElement).focus()
        }}>
          {([{view:'editor',label:'编辑器',Icon:Code2},{view:'git',label:'Git',Icon:GitBranch},{view:'skills',label:'Skills',Icon:Puzzle},{view:'settings',label:'设置',Icon:Settings2}] as const).map(({view,label,Icon}) => (
            <button key={view} id={'tab-'+view} className="folder-tab" role="tab" aria-label={label} title={label} aria-selected={activeView===view} aria-controls={'panel-'+view} tabIndex={activeView===view?0:-1} onClick={() => {cancelContinuation();setActiveView(view)}}>
              <Icon size={17} aria-hidden="true"/>
              <span>{label}</span>
            </button>
          ))}
        </div>
        <div className="top-spacer" />
        <button
          className={`icon-button ${actions.pinned ? 'active' : ''}`}
          aria-label={actions.pinned ? '取消置顶' : '置顶窗口'}
          title={actions.pinned ? '取消置顶' : '置顶窗口'}
          onClick={actions.togglePinned}
        >
          <Pin size={15} />
        </button>
        {windowControls}
      </header>
      <section id="panel-editor" role="tabpanel" aria-labelledby="tab-editor" className="editor-page" hidden={activeView!=='editor'}>
        <EditorWorkspace draft={draft} drafts={drafts} commands={commands} execution={execution} actions={actions} Editor={Editor} onPublish={() => publish()} onSettings={() => openSettings('runtimes')} setError={setError} />
      </section>
      <section id="panel-git" role="tabpanel" aria-labelledby="tab-git" className="git-page" hidden={activeView!=='git'}>
        {GitView ? <GitView active={activeView==='git'}/> : <div className="panel-loading">{gitLoadError ? <><span>{gitLoadError}</span><Button variant="outline" onClick={retryGit}>重试</Button></> : <><LoaderCircle size={18} className="animate-spin"/>正在加载 Git 视图…</>}</div>}
      </section>
      <section id="panel-skills" role="tabpanel" aria-labelledby="tab-skills" className="skills-page" hidden={activeView!=='skills'}>
        {SkillsView ? <SkillsView active={activeView==='skills'}/> : <div className="panel-loading">{skillsLoadError ? <><span>{skillsLoadError}</span><Button variant="outline" onClick={retrySkills}>重试</Button></> : <><LoaderCircle size={18} className="animate-spin"/>正在加载 Skills…</>}</div>}
      </section>
      <section id="panel-settings" role="tabpanel" aria-labelledby="tab-settings" className="settings-page" hidden={activeView!=='settings'}>
        {SettingsView ? <SettingsView
          active={activeView==='settings'}
          category={settingsCategory}
          onCategoryChange={setSettingsCategory}
          settings={settings}
          tokenSet={tokenSet}
          runtimes={runtimes}
          detecting={detecting}
          onDetect={detect}
          onClose={() => {
            cancelContinuation()
            setActiveView('editor')
          }}
          onTokenRemoved={() => setTokenSet(false)}
          onSaved={(value, hasToken) => {
            setSettings(value)
            setTokenSet(hasToken)
            void detect()
            if (resumeAfterSettings(value, hasToken)) setActiveView('editor')
          }}
        /> : <div className="panel-loading">{settingsLoadError ? <><span role="alert">{settingsLoadError}</span><Button variant="outline" onClick={retrySettings}>重试</Button></> : <><LoaderCircle size={18} className="animate-spin"/>正在加载设置…</>}</div>}
      </section>
      {closeDialog}
      <PublishOverlay publishing={publishing} settings={settings} />
      <PendingDialog actions={actions} />
      <DraftDialogs drafts={drafts} commands={commands} switching={actions.switching} run={execution.run} publish={() => publish()} setError={setError} />
    </div></SettingsContext.Provider>
}
