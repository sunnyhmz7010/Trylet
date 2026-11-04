import { useEffect, useRef, useState } from 'react'
import type { Draft, Settings } from '../../../bindings/trylet/internal/store/models'
import type { PublishRecord } from '../../../bindings/trylet/internal/publish/models'
import { errorText, resolvedLanguage } from '../../lib/language'
import type { DraftSession } from '../editor/useDraft'
import type { Dispatch, SetStateAction } from 'react'
type ErrorHandler = Dispatch<SetStateAction<string>>

export function usePublishing(
  drafts: DraftSession,
  settings: Settings | null,
  tokenSet: boolean,
  setError: ErrorHandler,
  setLanguageAction: (action: 'publish') => void,
  openSettings: () => void,
) {
  const { draftRef, transitionLock, flush } = drafts
  const [publishDraft, setPublishDraft] = useState<Draft | null>(null)
  const [PublishView, setPublishView] = useState<typeof import('./PublishDialog').default | null>(null)
  const [publishLoadError, setPublishLoadError] = useState('')
  useEffect(() => {
    if (!publishDraft || PublishView || publishLoadError) return
    void import('./PublishDialog').then((module) => setPublishView(() => module.default)).catch((failure) => setPublishLoadError(errorText(failure)))
  }, [publishDraft, PublishView, publishLoadError])
  const publishAfterSettings = useRef(false)
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
      openSettings()
      return
    }
    try {
      const snapshot = await flush()
      if (snapshot && draftRef.current?.id === snapshot.id && !transitionLock.current)
        setPublishDraft({ ...snapshot, language: lang })
    } catch (failure) {
      setError(errorText(failure))
    }
  }
  const published = (record: PublishRecord) => drafts.published(record, publishDraft?.id)
  return { publishDraft, setPublishDraft, PublishView, publishLoadError, setPublishLoadError, publishAfterSettings, publish, published }
}
export type PublishingSession = ReturnType<typeof usePublishing>
