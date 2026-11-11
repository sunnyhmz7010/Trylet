import { useRef, type ReactNode } from 'react'
import { LoaderCircle } from 'lucide-react'
import { Button } from '../ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../ui/dialog'

type Props = { open: boolean; title: string; description: ReactNode; children?: ReactNode; busy?: boolean;
  error?: string; confirmLabel: string; cancelLabel?: string; destructive?: boolean; onCancel: () => void; onConfirm: () => void }
export function ConfirmDialog({ open, title, description, children, busy = false, error, confirmLabel,
  cancelLabel = '取消', destructive = false, onCancel, onConfirm }: Props) {
  const cancel = useRef<HTMLButtonElement>(null)
  return <Dialog open={open} onOpenChange={(value) => { if (!value && !busy) onCancel() }}>
    <DialogContent data-operation-busy={busy} size="sm" className="dialog-content" showCloseButton={!busy}
      onOpenAutoFocus={(event) => { event.preventDefault(); cancel.current?.focus() }}
      onEscapeKeyDown={(event) => { if (busy) event.preventDefault() }}>
      <div className="dialog-heading"><DialogTitle>{title}</DialogTitle><DialogDescription>{description}</DialogDescription></div>
      {(children || error) && <div className="dialog-scroll">{children}{error && <p className="form-error" role="alert">{error}</p>}</div>}
      <div className="dialog-footer">
        <Button ref={cancel} variant="outline" disabled={busy} onClick={onCancel}>{cancelLabel}</Button>
        <Button variant={destructive ? 'destructive' : 'default'} disabled={busy} onClick={onConfirm}>{busy && <LoaderCircle size={13} className="animate-spin"/>}{busy ? '正在处理…' : confirmLabel}</Button>
      </div>
    </DialogContent>
  </Dialog>
}
