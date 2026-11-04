import { Button } from '../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../components/ui/dialog'
import type { WorkspaceActions } from './useWorkspaceActions'
export function CloseDialog({ actions }: { actions: WorkspaceActions }) {
  const { closePrompt, setClosePrompt, closeBusy, rememberClose, setRememberClose, closeError, chooseClose } = actions
  return     <Dialog open={closePrompt} onOpenChange={(open) => { if (!closeBusy) setClosePrompt(open) }}>
      <DialogContent className="dialog-content" aria-describedby="close-description">
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
export function PendingDialog({ actions }: { actions: WorkspaceActions }) {
  const { pending, setPending, switching, executeAction } = actions
  return       <Dialog
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
}
