import { Button } from '../../components/ui/button'
import { Input } from '../../components/ui/input'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../../components/ui/dialog'
import { languageNames, templates, type Language } from '../../lib/language'
import type { DraftSession } from './useDraft'
import type { EditorCommands } from './useEditorCommands'
type Props = {
  drafts: DraftSession
  commands: EditorCommands
  switching: boolean
  run: () => Promise<void>
  publish: () => Promise<void>
  setError: (error: string) => void
}
export default function DraftDialogs({ drafts, commands, switching, run, publish, setError }: Props) {
  const { draftRef, updateDraft } = drafts
  const { languageAction, setLanguageAction, manualLanguage, setManualLanguage, rename, setRename, renameOwner, changeLanguage } = commands
  return <>      <Dialog
        open={languageAction !== null}
        onOpenChange={(open) => {
          if (!open) setLanguageAction(null)
        }}
      >
        <DialogContent size="sm" className="dialog-content">
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
        <DialogContent size="sm" className="dialog-content">
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
      </Dialog></>
}
