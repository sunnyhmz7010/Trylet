import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import * as Backend from '../../../bindings/trylet/internal/app/service'
import * as Desktop from '../../../bindings/trylet/internal/desktop/desktop'
import type { Draft, DraftSummary } from '../../../bindings/trylet/internal/store/models'
import type { PublishRecord } from '../../../bindings/trylet/internal/publish/models'
import { errorText, resolvedLanguage } from '../../lib/language'
import type { Dispatch, SetStateAction } from 'react'
type ErrorHandler = Dispatch<SetStateAction<string>>

export function useDraft(setError: ErrorHandler) {
  const [draft, setDraft] = useState<Draft | null>(null)
  const draftRef = useRef<Draft | null>(null)
  const [history, setHistory] = useState<DraftSummary[]>([])
  const [saveStatus, setSaveStatus] = useState('已保存')
  const [exporting, setExporting] = useState(false)
  const transitionLock = useRef(false)
  const version = useRef(0), savedVersion = useRef(0), queuedVersion = useRef(0)
  const saveChain = useRef<Promise<Draft | undefined>>(Promise.resolve(undefined))
  const saveFailure = useRef('')
  const updateDraft = useCallback((patch: Partial<Draft>) => {
    if (!draftRef.current || transitionLock.current) return false
    const next = { ...draftRef.current, ...patch }
    draftRef.current = next
    version.current++
    setDraft(next)
    setSaveStatus('待保存')
    return true
  }, [])
  // 保存排队执行，只确认已写入的版本，保留请求期间的新编辑。
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
          if (saveFailure.current) {
            const recovered = saveFailure.current
            setError((current) => current === recovered ? '' : current)
            saveFailure.current = ''
          }
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
        saveFailure.current = errorText(failure)
        setError(saveFailure.current)
        throw failure
      }
    }
    saveChain.current = saveChain.current.then(operation, operation)
    await saveChain.current
    return snapshot
  }, [])
  useEffect(() => {
    if (!draft || version.current === savedVersion.current) return
    const timer = window.setTimeout(() => {
      void flush().catch(() => {
        /* 错误已由保存入口显示，保留待保存内容。 */
      })
    }, 400)
    return () => clearTimeout(timer)
  }, [draft?.code, draft?.language, draft?.title, draft?.id, flush])
  const language = useMemo(() => draft ? resolvedLanguage(draft.language, draft.code) : null, [draft?.language, draft?.code])
  const hydrateDraft = (value: Draft) => {
    draftRef.current = value
    version.current = 0
    queuedVersion.current = 0
    savedVersion.current = 0
    setDraft(value)
    setSaveStatus('已保存')
  }
  const restore = useCallback((value: Draft, items: DraftSummary[]) => {
    draftRef.current = value
    setDraft(value)
    setHistory(items ?? [])
  }, [])
  const refreshHistory = useCallback(async () => setHistory((await Backend.History()) ?? []), [])
  const exportDraft = async () => {
    if (exporting || transitionLock.current) return
    setExporting(true)
    try {
      const snapshot = await flush()
      if (snapshot) {
        const path = await Desktop.ExportDraft(snapshot.id, resolvedLanguage(snapshot.language, snapshot.code) ?? 'auto')
        if (path) setSaveStatus('已导出')
      }
    } catch (failure) {
      setError(errorText(failure))
    } finally {
      setExporting(false)
    }
  }
  const published = (record: PublishRecord, owner: string | undefined) => {
    const current = draftRef.current
    if (!current || current.id !== owner) return
    const title =
      current.title === '未命名练习' ? record.target.path.split('/').at(-1)! : current.title
    draftRef.current = { ...current, title, publish: record }
    setDraft(draftRef.current)
    void Backend.History()
      .then((v) => setHistory(v ?? []))
      .catch((failure) => setError(errorText(failure)))
  }
  return {
    draft, draftRef, history, saveStatus, language, transitionLock, exporting,
    restore, hydrateDraft, updateDraft, flush, refreshHistory, exportDraft, published,
  }
}
export type DraftSession = ReturnType<typeof useDraft>
