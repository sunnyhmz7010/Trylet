import { useState } from 'react'
import { performDraftTransition, type PendingAction } from './draftTransition'
import { useWindowActions } from './useWindowActions'
import { useWorkspaceShortcuts } from './useWorkspaceShortcuts'
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

export function useWorkspaceActions(
  drafts: Pick<DraftSession, 'transitionLock' | 'flush' | 'hydrateDraft' | 'refreshHistory'>,
  execution: Pick<ExecutionSession, 'phaseRef' | 'activeRun' | 'run' | 'reset'>,
  publishing: Pick<PublishingSession, 'publish'>,
  settings: Settings | null,
  setSettings: Dispatch<SetStateAction<Settings | null>>,
  setError: ErrorHandler,
  activeView: View,
) {
  const { transitionLock, flush, hydrateDraft, refreshHistory } = drafts
  const { phaseRef, activeRun, run } = execution
  const { publish } = publishing
  const [pending, setPending] = useState<PendingAction | null>(null)
  const [switching, setSwitching] = useState(false)
  const executeAction = async (action: PendingAction) => {
    if (transitionLock.current || phaseRef.current === 'preparing') return
    transitionLock.current = true
    setSwitching(true)
    try {
      await performDraftTransition(action, {
        save: flush,
        stop: async () => { if (activeRun.current) await Backend.StopRun(activeRun.current) },
        quit: Desktop.Quit,
        create: Backend.NewDraft,
        load: Backend.LoadDraft,
        activate: (value) => { hydrateDraft(value); execution.reset() },
        refresh: refreshHistory,
      })
      setError('')
    } catch (failure) {
      setError(errorText(failure))
      if (action.kind === 'quit') {
        windowActions.showCloseError(errorText(failure))
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
      setError(action.kind === 'quit' ? '正在准备运行，请稍候再退出。' : '正在准备运行，请稍候再切换练习。')
      return
    }
    if (activeRun.current) setPending(action)
    else void executeAction(action)
  }
  const windowActions = useWindowActions(flush, () => requestAction({ kind: 'quit' }),
    () => phaseRef.current !== 'preparing' && !transitionLock.current,
    settings, setSettings, setError)
  useWorkspaceShortcuts({ activeView, run, publish, hide: windowActions.hide,
    create: () => requestAction({ kind: 'new' }),
    save: async () => { try { await flush() } catch (failure) { setError(errorText(failure)) } },
  })
  return { pending, setPending, switching, requestAction, executeAction, ...windowActions }
}
export type WorkspaceActions = ReturnType<typeof useWorkspaceActions>
