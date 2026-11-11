import { useRef, useState } from 'react'
import type { Draft, Settings } from '../../../bindings/trylet/internal/store/models'
import type { PublishRecord } from '../../../bindings/trylet/internal/publish/models'
import { errorText, resolvedLanguage } from '../../lib/language'
import type { DraftSession } from '../editor/useDraft'
import type { Dispatch, SetStateAction } from 'react'
import { useDeferredModule } from '../../lib/useDeferredModule'
import { PublishContinuation } from './publishContinuation'

const loadPublish = () => import('./PublishDialog')

type ErrorHandler = Dispatch<SetStateAction<string>>

export function usePublishing(
  drafts: Pick<DraftSession, 'draftRef' | 'transitionLock' | 'flush' | 'published'>,
  settings: Settings | null,
  tokenSet: boolean,
  setError: ErrorHandler,
  setLanguageAction: (action: 'publish') => void,
  openSettings: () => void,
) {
  const { draftRef, transitionLock, flush } = drafts
  const [publishDraft, setPublishDraft] = useState<Draft | null>(null)
  const { component: PublishView, error: publishLoadError, retry: retryPublish } = useDeferredModule(!!publishDraft, loadPublish)
  const continuation = useRef(new PublishContinuation())
  const preparing = useRef(false)
  const cancelContinuation = () => continuation.current.cancel()
  const publish = async (newSettings = settings, hasToken = tokenSet) => {
    if (transitionLock.current || preparing.current || publishDraft) return
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
      continuation.current.request(current.id)
      openSettings()
      return
    }
    cancelContinuation()
    preparing.current = true
    try {
      const snapshot = await flush()
      if (snapshot && draftRef.current?.id === snapshot.id && !transitionLock.current)
        setPublishDraft({ ...snapshot, language: lang })
    } catch (failure) {
      setError(errorText(failure))
    } finally {
      preparing.current = false
    }
  }
  const resumeAfterSettings = (value: Settings, hasToken: boolean) => {
    if (!continuation.current.consume(draftRef.current?.id, !!value.repository && hasToken)) return false
    void publish(value, hasToken)
    return true
  }
  const published = (record: PublishRecord) => drafts.published(record, publishDraft?.id)
  return { publishDraft, setPublishDraft, PublishView, publishLoadError, retryPublish, cancelContinuation, resumeAfterSettings, publish, published }
}
export type PublishingSession = ReturnType<typeof usePublishing>
