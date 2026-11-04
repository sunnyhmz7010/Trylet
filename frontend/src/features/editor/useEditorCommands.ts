import { useRef, useState } from 'react'
import type { Draft } from '../../../bindings/trylet/internal/store/models'
import type { Settings } from '../../../bindings/trylet/internal/store/models'
import { templates, type Language } from '../../lib/language'
import type { DraftSession } from './useDraft'
import type { Dispatch, SetStateAction } from 'react'
type ErrorHandler = Dispatch<SetStateAction<string>>
export type LanguageAction = 'run' | 'publish' | 'template'

export function useEditorCommands(drafts: DraftSession, settings: Settings | null, setError: ErrorHandler) {
  const { draftRef, language, updateDraft } = drafts
  const [languageAction, setLanguageAction] = useState<LanguageAction | null>(null)
  const [manualLanguage, setManualLanguage] = useState<Language>('python')
  const [rename, setRename] = useState<string | null>(null)
  const renameOwner = useRef('')
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
  return { languageAction, setLanguageAction, manualLanguage, setManualLanguage, rename, setRename, renameOwner, changeLanguage, insertTemplate }
}
export type EditorCommands = ReturnType<typeof useEditorCommands>
