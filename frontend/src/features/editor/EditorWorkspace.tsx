import { ChevronDown, Code2, FolderClock, Github, LoaderCircle, Minus, Play, Plus, Square, TerminalSquare, Trash2 } from 'lucide-react'
import type { Dispatch, SetStateAction } from 'react'
import type { Draft } from '../../../bindings/trylet/internal/store/models'
import { Button } from '../../components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuTrigger } from '../../components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../components/ui/select'
import { errorText, languageNames } from '../../lib/language'
import type { DraftSession } from './useDraft'
import type { EditorCommands } from './useEditorCommands'
import type { ExecutionSession } from './useExecution'
import type { WorkspaceActions } from '../../app/useWorkspaceActions'

type Props = {
  draft: Draft
  drafts: DraftSession
  commands: EditorCommands
  execution: ExecutionSession
  actions: Pick<WorkspaceActions, 'switching' | 'requestAction'>
  Editor: typeof import('./Editor').default
  onPublish: () => Promise<void>
  onSettings: () => void
  setError: Dispatch<SetStateAction<string>>
}
export default function EditorWorkspace({ draft, drafts, commands, execution, actions, Editor, onPublish, onSettings, setError }: Props) {
  const { history, language, saveStatus, updateDraft, exportDraft, exporting, refreshHistory } = drafts
  const { switching, requestAction } = actions
  const { changeLanguage, insertTemplate, renameOwner, setRename } = commands
  const { Terminal, terminalOpen, setTerminalOpen, terminalHeight, resizeTerminal, onTerminalReady, phase, busy, runMessage, selectedRuntime, detecting, run } = execution
  return <>      <header className="page-toolbar">
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
              void refreshHistory()
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
            <DropdownMenuItem disabled={switching || exporting} onSelect={() => void exportDraft()}>
              导出当前草稿…
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
                    <span>{languageNames[item.language]}</span>
                    <span>{new Date(item.updatedAt).toLocaleTimeString('zh-CN', {
                      hour: '2-digit',
                      minute: '2-digit',
                    })}</span>
                  </span>
                </div>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>

      </header>
      <main className="workspace">
        <div className="editor-holder">
          <Editor
            key={draft.id}
            value={draft.code}
            language={language}
            locked={switching}
            onChange={(value) => updateDraft({ code: value })}
            onError={setError}
          />
          {!draft.code && <div className="editor-hint">选择语言可填入模板，Ctrl+Enter 运行</div>}
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
                onClick={() => execution.clearTerminal()}
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
<Terminal onReady={onTerminalReady} onInput={execution.onInput} onResize={execution.onResize} />
          </section>
        )}
      </main>
      <footer className="footer page-footer">
        <button
          className="environment"
          title={
            selectedRuntime ? `${selectedRuntime.name}：${selectedRuntime.path}` : '选择本机环境'
          }
          onClick={() => {onSettings()}}
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
        {drafts.saveError ? <button className="text-action footer-status" title={drafts.saveError} disabled={switching || saveStatus === '保存中'} onClick={() => void drafts.flush().catch((failure) => setError(errorText(failure)))}>{saveStatus === '保存中' ? '正在重试保存…' : '保存失败，重试'}</button> : <span className="footer-status">{saveStatus}</span>}
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
            onClick={() => void onPublish()}
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
      </footer></>
}
