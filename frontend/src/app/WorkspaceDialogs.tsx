import { ConfirmDialog } from '../components/feedback/ConfirmDialog'
import { Button } from '../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../components/ui/dialog'
import type { WindowActions } from './useWindowActions'
import type { WorkspaceActions } from './useWorkspaceActions'
export function CloseDialog({ actions }: { actions: WindowActions }) {
  const { closePrompt, setClosePrompt, closeBusy, rememberClose, setRememberClose, closeError, chooseClose } = actions
  return     <Dialog open={closePrompt} onOpenChange={(open) => { if (!closeBusy) setClosePrompt(open) }}>
      <DialogContent className="dialog-content" aria-describedby="close-description" showCloseButton={!closeBusy} onEscapeKeyDown={(event) => { if (closeBusy) event.preventDefault() }}>
        <div className="dialog-heading">
          <DialogTitle>关闭 Trylet 时要怎么做？</DialogTitle>
          <DialogDescription id="close-description">
            退出应用会结束后台进程；最小化到托盘会继续在后台运行。
          </DialogDescription>
        </div>
        <div className="dialog-scroll">
          <label className="check-field">
            <input type="checkbox" checked={rememberClose} disabled={closeBusy} onChange={(event) => setRememberClose(event.target.checked)} />
            记住我的选择，下次不再询问
          </label>
          {closeError && <p className="form-error" role="alert">{closeError}</p>}
        </div>
        <div className="dialog-footer">
          <Button variant="outline" disabled={closeBusy} onClick={() => void chooseClose('background')}>最小化到托盘</Button>
          <Button variant="destructive" disabled={closeBusy} onClick={() => void chooseClose('quit')}>退出应用</Button>
        </div>
      </DialogContent>
    </Dialog>
}
export function PendingDialog({ actions }: { actions: Pick<WorkspaceActions, 'pending' | 'setPending' | 'switching' | 'executeAction'> }) {
  const { pending, setPending, switching, executeAction } = actions
  return <ConfirmDialog open={pending !== null} title="停止当前运行？"
    description={`当前练习仍在运行。代码会先保存，然后停止运行并${pending?.kind === 'quit' ? '退出应用' : '切换练习'}。`}
    busy={switching} confirmLabel="停止并继续" onCancel={() => setPending(null)}
    onConfirm={() => { if (pending) void executeAction(pending) }} />
}
