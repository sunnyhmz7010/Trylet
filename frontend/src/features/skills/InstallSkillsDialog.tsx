import { useEffect, useRef, useState } from 'react'
import { Events } from '@wailsio/runtime'
import { AlertCircle, Check, LoaderCircle } from 'lucide-react'
import * as Backend from '../../../bindings/trylet/internal/skills/service'
import type { SkillDownloadProgress, SkillInstallResult, SkillLocation, SkillPreview } from '../../../bindings/trylet/internal/skills/models'
import { Button } from '../../components/ui/button'
import { Input } from '../../components/ui/input'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '../../components/ui/dialog'
import { normalizeInstallResults } from './installResults'
import { errorText, sessionID } from '../../lib/language'

export default function InstallSkillsDialog({ location, onClose, onChanged }: { location: SkillLocation; onClose: () => void; onChanged: () => void }) {
  const [repository, setRepository] = useState('')
  const [extract, setExtract] = useState(true)
  const [preview, setPreview] = useState<SkillPreview | null>(null)
  const [selected, setSelected] = useState<string[]>([])
  const [results, setResults] = useState<SkillInstallResult[]>([])
  const [busy, setBusyState] = useState<'preview' | 'install' | 'reset' | ''>('')
  const busyRef = useRef(busy)
  const setBusy = (value: typeof busy) => { busyRef.current = value; setBusyState(value) }
  const cancellingRef = useRef(false)
  const [error, setError] = useState('')
  const [progress, setProgress] = useState('')
  const [cancelling, setCancelling] = useState(false)
  const previewRef = useRef('')
  const request = useRef<ReturnType<typeof Backend.PrepareSkillInstall> | null>(null)
  const mounted = useRef(true)
  useEffect(() => Events.On('trylet:skill-download', (event) => {
    const data = event.data as SkillDownloadProgress
    if (data.id !== previewRef.current) return
    const phases: Record<string, string> = { connecting: '正在连接 GitHub…', downloading: '正在下载仓库…', extracting: '正在解压仓库…', scanning: '正在识别 Skills…' }
    setProgress(phases[data.phase] ?? '')
  }), [])
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      void request.current?.cancel()
      if (previewRef.current) void Backend.DiscardSkillPreview(previewRef.current).catch((failure) => console.error('清理 Skills 下载预览失败', failure))
    }
  }, [])
  const prepare = async () => {
    if (busyRef.current || cancellingRef.current) return
    setBusy('preview'); setError(''); setProgress('')
    const id = sessionID()
    previewRef.current = id
    const pending = Backend.PrepareSkillInstall(id, repository, extract)
    request.current = pending
    try {
      const next = await pending
      if (!mounted.current || previewRef.current !== id) return
      setPreview(next)
      setSelected(next.mode === 'repository' ? ['.'] : next.candidates?.length === 1 ? [next.candidates[0].path] : [])
      setResults([])
      setProgress('')
    } catch (failure) {
      if (mounted.current && previewRef.current === id) {
        previewRef.current = ''
        void Backend.DiscardSkillPreview(id).catch((cleanupError) => console.error('清理失败的 Skills 下载预览', cleanupError))
        setError(errorText(failure)); setProgress('')
      }
    } finally {
      if (request.current === pending) {
        request.current = null
        if (mounted.current) setBusy('')
      }
    }
  }
  const cancelDownload = async (close = false) => {
    if (cancellingRef.current || busyRef.current === 'install' || busyRef.current === 'reset') return
    cancellingRef.current = true
    const id = previewRef.current
    previewRef.current = ''
    setCancelling(true)
    try {
      await request.current?.cancel()
      if (id) await Backend.DiscardSkillPreview(id)
      if (mounted.current) {
        setProgress('已取消下载')
        if (close) onClose()
      }
    } catch (failure) {
      if (mounted.current) { previewRef.current = id; setError(errorText(failure)) }
    } finally {
      cancellingRef.current = false
      if (mounted.current) { setBusy(''); setCancelling(false) }
    }
  }
  const install = async () => {
    if (!preview || busyRef.current || selected.length === 0) return
    setBusy('install'); setError('')
    try {
      const next = normalizeInstallResults(selected, (await Backend.InstallSkills(preview.id, location.id, selected)) ?? [])
      setResults((current) => [...current.filter((item) => !next.some((result) => result.sourcePath === item.sourcePath)), ...next])
      const successful = next.filter((item) => !item.error)
      if (successful.length) onChanged()
      setSelected(next.filter((item) => item.error).map((item) => item.sourcePath))
    } catch (failure) { setError(errorText(failure)) }
    finally { setBusy('') }
  }
  const reset = async () => {
    if (!preview || busyRef.current) return
    setBusy('reset')
    try {
      await Backend.DiscardSkillPreview(preview.id)
      previewRef.current = ''; setPreview(null); setSelected([]); setResults([]); setError('')
    } catch (failure) { setError(errorText(failure)) }
    finally { setBusy('') }
  }
  const completed = new Set(results.filter((item) => !item.error).map((item) => item.sourcePath))
  const available = preview?.candidates?.filter((item) => !completed.has(item.path)) ?? []
  const canClose = !cancelling && busy !== 'install' && busy !== 'reset'
  const close = () => { if (canClose) { if (previewRef.current) void cancelDownload(true); else onClose() } }
  return <Dialog open onOpenChange={(open) => { if (!open) close() }}>
    <DialogContent size="md" data-operation-busy={!canClose} className="skills-dialog" showCloseButton={canClose} onEscapeKeyDown={(event) => { if (!canClose) event.preventDefault() }}>
      <DialogTitle>安装 Skill</DialogTitle>
      <DialogDescription className="skill-full-path">安装到 {location.path}</DialogDescription>
      <ol className="skills-steps" aria-label="安装步骤">{['输入来源', '选择技能', '安装结果'].map((label, index) => <li key={label} aria-current={(results.length ? 2 : preview ? 1 : 0) === index ? 'step' : undefined}>{index + 1}. {label}</li>)}</ol>
      <form className="skills-download-form" onSubmit={(event) => { event.preventDefault(); void (preview ? install() : prepare()) }}>
        <div className="skills-dialog-body">
          {!preview && <>
            <div className="field"><label htmlFor="skill-repository">GitHub 仓库</label><Input id="skill-repository" autoFocus placeholder="https://github.com/owner/repo" value={repository} disabled={!!busy} onChange={(event) => setRepository(event.target.value)} required/></div>
            <fieldset className="skills-install-mode" disabled={!!busy}><legend>安装方式</legend>
              <label className="skills-checkbox"><input type="radio" name="install-mode" checked={extract} onChange={() => setExtract(true)}/>选择仓库中的技能</label>
              <label className="skills-checkbox"><input type="radio" name="install-mode" checked={!extract} onChange={() => setExtract(false)}/>安装整个仓库</label>
            </fieldset>
            <p className="skills-help">{extract ? '下一步勾选需要的技能，各自安装到独立目录。' : '保留完整仓库结构，适合依赖共享文件的技能；以后整体更新或删除。'}</p>
          </>}
          {results.length > 0 && <p className="skills-result" role="status">已安装 {completed.size} 项{selected.length ? `，${selected.length} 项待重试。已成功的项目不会重复安装。` : '，安装完成。'}</p>}
          {preview && <>
            <div className="skills-preview-heading"><strong>{preview.repository}</strong><span>{preview.mode === 'repository' ? '整仓库安装' : `${preview.candidates?.length ?? 0} 个候选技能`}</span></div>
            {!results.length && preview.mode === 'extracted' && <label className="skills-checkbox"><input type="checkbox" disabled={!!busy || !available.length} checked={available.length > 0 && selected.length === available.length} onChange={(event) => setSelected(event.target.checked ? available.map((item) => item.path) : [])}/>全选</label>}
            {!preview.candidates?.length && <p className="skills-help">此仓库未识别到可安装的技能。请返回上一步检查来源，或选择安装整个仓库。</p>}
            <div className="skills-candidates">
              {preview.candidates?.map((item) => { const result = results.find((result) => result.sourcePath === item.path); return <div className="skill-candidate" key={item.path}>
                <label className="skills-checkbox"><input type="checkbox" checked={completed.has(item.path) || selected.includes(item.path)} disabled={!!busy || !!results.length || completed.has(item.path)} onChange={(event) => setSelected((current) => event.target.checked ? [...current, item.path] : current.filter((path) => path !== item.path))}/><span><strong>{item.name}</strong><code>{item.path === '.' ? '仓库根目录' : item.path}</code></span>{completed.has(item.path) && <Check size={14} className="success"/>}</label>
                {result?.warning && <p className="skills-help">{result.warning}</p>}
                {result?.error && <p className="skill-error" role="alert">{result.error}</p>}
              </div> })}
            </div>
          </>}
          {error && <p className="form-error skills-download-error" role="alert"><AlertCircle size={13}/>{error}</p>}
          {progress && !error && <p className="skills-help" role="status">{cancelling ? '正在取消下载…' : progress}</p>}
        </div>
        <div className="dialog-footer">
          {!(results.length > 0 && !selected.length) && <Button type="button" variant="outline" disabled={!canClose} onClick={close}>{cancelling ? '正在取消…' : results.length ? '关闭' : '取消安装'}</Button>}
          {preview && !results.length && <Button type="button" variant="outline" disabled={!!busy} onClick={() => void reset()}>上一步</Button>}
          {results.length > 0 && !selected.length ? <Button type="button" disabled={!canClose} onClick={close}>{cancelling ? '正在关闭…' : '完成'}</Button> :
            <Button type="submit" disabled={!!busy || cancelling || (!preview && !repository.trim()) || (!!preview && !selected.length)}>
              {busy && <LoaderCircle size={13} className="animate-spin"/>}
              {busy === 'preview' ? '正在获取技能…' : busy === 'install' ? '正在安装…' : busy === 'reset' ? '正在返回…' : results.length ? `重试失败项（${selected.length}）` : preview ? `安装所选（${selected.length}）` : '下一步'}
            </Button>}
        </div>
      </form>
    </DialogContent>
  </Dialog>
}
