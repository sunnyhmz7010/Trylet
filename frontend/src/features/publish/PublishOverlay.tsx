import { Button } from '../../components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../../components/ui/dialog'
import type { Settings } from '../../../bindings/trylet/internal/store/models'
import type { PublishingSession } from './usePublishing'
export default function PublishOverlay({ publishing, settings }: { publishing: PublishingSession; settings: Settings }) {
  const { publishDraft, setPublishDraft, PublishView, publishLoadError, retryPublish, published } = publishing
  return <>      {publishDraft && (PublishView ? (
        <PublishView
          draft={publishDraft}
          settings={settings}
          onClose={() => setPublishDraft(null)}
          onPublished={published}
        />
      ) : <Dialog open onOpenChange={(open) => { if (!open) setPublishDraft(null) }}>
        <DialogContent className="dialog-content">
          <div className="dialog-heading"><DialogTitle>发布到 GitHub</DialogTitle><DialogDescription>{publishLoadError ? '发布界面加载失败，请重试。' : '正在加载发布界面…'}</DialogDescription></div>
          {publishLoadError && <div className="dialog-scroll"><p className="form-error" role="alert">{publishLoadError}</p></div>}
          <div className="dialog-footer"><Button variant="outline" onClick={() => setPublishDraft(null)}>取消</Button>{publishLoadError && <Button onClick={retryPublish}>重试</Button>}</div>
        </DialogContent>
      </Dialog>)}</>
}
