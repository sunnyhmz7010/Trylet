import { useCallback, useEffect, useRef, useState } from 'react'
import { AlertCircle, Check, ChevronDown, ChevronRight, Download, FolderOpen, Link2, LoaderCircle, Plus, RefreshCw, Trash2 } from 'lucide-react'
import * as Backend from '../bindings/trylet/internal/trylet/service'
import * as Desktop from '../bindings/trylet/desktop'
import type { SkillInstallation, SkillInstallResult, SkillLocation, SkillPreferences, SkillPreview, SkillUpdateResult, SkillWorkspace } from '../bindings/trylet/internal/trylet/models'
import { Button } from './components/ui/button'
import { Input } from './components/ui/input'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from './components/ui/dialog'
import { errorText } from './lib/language'
import './skills.css'

const toolNames: Record<string, string> = { agents: '共享 · .agents', codex: 'Codex · .codex', claude: 'Claude Code · .claude', custom: '自定义' }

export default function SkillsPanel({ active }: { active: boolean }) {
  const [workspace, setWorkspace] = useState<SkillWorkspace | null>(null)
  const [contextBusy, setContextBusy] = useState(false)
  const [workspaceError, setWorkspaceError] = useState('')
  const [snapshot, setSnapshot] = useState<{ locationID: string; items: SkillInstallation[] }>({ locationID: '', items: [] })
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState('')
  const [query, setQuery] = useState('')
  const [expanded, setExpanded] = useState<Set<string>>(new Set())
  const [busy, setBusy] = useState<Set<string>>(new Set())
  const [itemErrors, setItemErrors] = useState<Record<string, string>>({})
  const [notice, setNotice] = useState({ locationID: '', message: '' })
  const [downloadLocation, setDownloadLocation] = useState<SkillLocation | null>(null)
  const [deleteItem, setDeleteItem] = useState<SkillInstallation | null>(null)
  const [overwrite, setOverwrite] = useState<{ item: SkillInstallation; result: SkillUpdateResult } | null>(null)
  const generation = useRef(0)
  const location = workspace?.locations?.find((item) => item.path === workspace.preferences.location && item.scope === workspace.preferences.scope)
  const locationRef = useRef(location?.id)
  locationRef.current = location?.id

  const boot = useCallback(async () => {
    setWorkspaceError('')
    try { setWorkspace(await Backend.GetSkillWorkspace()) }
    catch (failure) { setWorkspaceError(errorText(failure)) }
  }, [])
  useEffect(() => { void boot() }, [boot])

  const refresh = useCallback(async (id: string) => {
    if (locationRef.current !== id) return
    const request = ++generation.current
    setLoading(true)
    setLoadError('')
    try {
      const items = await Backend.ScanSkills(id)
      if (request === generation.current && locationRef.current === id) {
        setSnapshot({ locationID: id, items: items ?? [] })
      }
    } catch (failure) {
      if (request === generation.current && locationRef.current === id) setLoadError(errorText(failure))
    } finally {
      if (request === generation.current && locationRef.current === id) setLoading(false)
    }
  }, [])
  useEffect(() => {
    if (active && location?.id) void refresh(location.id)
    return () => { generation.current++ }
  }, [active, location?.id, refresh])

  const changeContext = async (patch: Partial<SkillPreferences>) => {
    if (!workspace || contextBusy) return
    setContextBusy(true)
    setWorkspaceError('')
    try { setWorkspace(await Backend.SaveSkillPreferences({ ...workspace.preferences, ...patch })) }
    catch (failure) { setWorkspaceError(errorText(failure)) }
    finally { setContextBusy(false) }
  }
  const chooseDirectory = async (custom: boolean) => {
    if (!workspace) return
    try {
      const path = await Desktop.ChooseSkillsDirectory()
      if (!path) return
      if (custom) {
        const project = workspace.preferences.scope === 'project' ? workspace.preferences.project : ''
        await changeContext({ location: path, customLocations: [...(workspace.preferences.customLocations ?? []).filter((item) => !(item.path.toLowerCase() === path.toLowerCase() && item.project.toLowerCase() === project.toLowerCase())), { path, project }] })
      } else await changeContext({ scope: 'project', project: path, location: '' })
    } catch (failure) { setWorkspaceError(errorText(failure)) }
  }
  const markBusy = (id: string, value: boolean) => setBusy((current) => {
    const next = new Set(current)
    if (value) next.add(id); else next.delete(id)
    return next
  })
  const update = async (item: SkillInstallation, confirmation = '') => {
    if (busy.has(item.id)) return
    markBusy(item.id, true)
    setItemErrors((current) => ({ ...current, [item.id]: '' }))
    try {
      const result = await Backend.UpdateSkill(item.id, confirmation)
      if (result.needsConfirmation) setOverwrite({ item, result })
      else {
        setOverwrite(null)
        setNotice({ locationID: item.locationID, message: `${item.name}：${result.warning || result.message}` })
        await refresh(item.locationID)
      }
    } catch (failure) { setItemErrors((current) => ({ ...current, [item.id]: errorText(failure) })) }
    finally { markBusy(item.id, false) }
  }
  const remove = async (item: SkillInstallation) => {
    if (busy.has(item.id)) return
    markBusy(item.id, true)
    setItemErrors((current) => ({ ...current, [item.id]: '' }))
    try {
      const warning = await Backend.DeleteSkill(item.locationID, item.name)
      setDeleteItem(null)
      setNotice({ locationID: item.locationID, message: warning || `已删除 ${item.name}` })
      await refresh(item.locationID)
    } catch (failure) { setItemErrors((current) => ({ ...current, [item.id]: errorText(failure) })) }
    finally { markBusy(item.id, false) }
  }
  const items = snapshot.locationID === location?.id ? snapshot.items : []
  const search = query.trim().toLowerCase()
  const filtered = items.filter((item) => `${item.name} ${item.repository} ${(item.skills ?? []).map((skill) => `${skill.name} ${skill.path}`).join(' ')}`.toLowerCase().includes(search))

  if (!workspace) return <div className="panel-loading">{workspaceError ? <><span role="alert">{workspaceError}</span><Button variant="outline" onClick={() => void boot()}>重试</Button></> : <><LoaderCircle size={18} className="animate-spin"/>正在读取 Skills 位置…</>}</div>
  return <div className="skills-inline">
    <header className="page-toolbar skills-toolbar">
      <nav className="section-tabs" aria-label="Skills 作用域">
        <button type="button" aria-pressed={workspace.preferences.scope === 'user'} disabled={contextBusy} onClick={() => void changeContext({ scope: 'user', location: '' })}>用户级</button>
        <button type="button" aria-pressed={workspace.preferences.scope === 'project'} disabled={contextBusy} onClick={() => void changeContext({ scope: 'project', location: '' })}>项目级</button>
      </nav>
      <div className="top-spacer"/>
      {workspace.preferences.scope === 'project' && <Button variant="outline" size="sm" disabled={contextBusy} onClick={() => void chooseDirectory(false)}><FolderOpen size={13}/>{workspace.preferences.project ? '更换项目' : '选择项目'}</Button>}
      <Button size="sm" disabled={!location || location.readOnly || contextBusy} title={location?.readOnly ? '系统内置目录只读' : '从 GitHub 下载 Skills'} onClick={() => setDownloadLocation(location!)}><Download size={13}/>下载</Button>
    </header>
    <div className="skills-location-bar">
      <label htmlFor="skills-location">安装位置</label>
      <select id="skills-location" className="native-select" value={location?.path ?? ''} disabled={contextBusy || !location} onChange={(event) => void changeContext({ location: event.target.value })}>
        {!location && <option value="">请先选择项目</option>}
        {workspace.locations?.filter((item) => item.scope === workspace.preferences.scope).map((item) => <option key={item.id} value={item.path}>{toolNames[item.tool]}{item.tool === 'custom' ? ` · ${item.path}` : ''}</option>)}
      </select>
      <button className="icon-button" type="button" title="添加自定义 Skills 目录" aria-label="添加自定义 Skills 目录" disabled={contextBusy || (workspace.preferences.scope === 'project' && !workspace.preferences.project)} onClick={() => void chooseDirectory(true)}><Plus size={15}/></button>
      <button className="icon-button" type="button" title="刷新技能列表" aria-label="刷新技能列表" disabled={!location || loading || contextBusy} onClick={() => location && void refresh(location.id)}><RefreshCw size={14} className={loading ? 'animate-spin' : ''}/></button>
    </div>
    {location && <div className="skills-location-path" title={location.path}>{location.path}</div>}
    {workspaceError && <div className="skills-feedback form-error" role="alert">{workspaceError}</div>}
    <div className="skills-workspace">
      <div className="skills-search"><Input aria-label="搜索技能名称或仓库" placeholder="搜索技能名称或仓库" value={query} onChange={(event) => setQuery(event.target.value)}/><span>{items.length} 项</span></div>
      {notice.locationID === location?.id && notice.message && <div className="skills-feedback" role="status">{notice.message}</div>}
      {loadError && <div className="skills-feedback form-error" role="alert">{loadError} <button type="button" className="text-action" onClick={() => location && void refresh(location.id)}>重试</button></div>}
      <div className="skills-list" aria-busy={loading}>
        {!location ? <div className="skills-empty"><FolderOpen size={22}/><strong>选择一个项目目录</strong><span>查看或安装项目使用的 Skills。</span><Button variant="outline" onClick={() => void chooseDirectory(false)}>选择项目</Button></div>
          : loading && items.length === 0 ? <div className="skills-empty"><LoaderCircle size={20} className="animate-spin"/>正在扫描 Skills…</div>
          : !loadError && filtered.length === 0 ? <div className="skills-empty"><strong>{search ? '没有匹配的技能' : '此位置还没有 Skills'}</strong><span>{search ? '换个名称或仓库试试。' : '从 GitHub 仓库下载，或切换安装位置。'}</span>{search && <Button variant="outline" onClick={() => setQuery('')}>清除搜索</Button>}</div>
          : filtered.map((item) => <article className="skill-row" key={item.id}>
            <div className="skill-row-heading">
              <button className="skill-name" type="button" aria-expanded={expanded.has(item.id)} onClick={() => setExpanded((current) => { const next = new Set(current); if (next.has(item.id)) next.delete(item.id); else next.add(item.id); return next })}>
                {expanded.has(item.id) ? <ChevronDown size={13}/> : <ChevronRight size={13}/>}<strong>{item.name}</strong>{item.linked && <Link2 size={13} aria-label="链接目录"/>}
              </button>
              <span className="skill-state">{item.readOnly ? '系统内置 · 只读' : item.linked ? '链接' : item.missing ? '目录缺失' : item.mode === 'git' ? `Git 克隆 · ${item.skills?.length ?? 0} 个技能` : item.mode === 'repository' ? `整仓库 · ${item.skills?.length ?? 0} 个技能` : `${item.skills?.length ?? 0} 个技能`}</span>
              <div className="skill-actions">
                {!item.readOnly && <><Button variant="ghost" size="xs" disabled={!item.repository || item.linked || (item.mode === 'git' && item.missing) || busy.has(item.id)} title={item.mode === 'git' ? '快进更新当前 Git 分支，保留本地改动' : item.repository ? '从来源仓库更新' : '未绑定仓库，无法更新'} onClick={() => void update(item)}>{busy.has(item.id) ? <LoaderCircle size={12} className="animate-spin"/> : <RefreshCw size={12}/>}更新</Button>
                  <button type="button" className="icon-button skill-delete" aria-label={`删除 ${item.name}`} title="删除" disabled={busy.has(item.id)} onClick={() => setDeleteItem(item)}><Trash2 size={13}/></button></>}
              </div>
            </div>
            <div className="skill-source">{item.repository ? <button type="button" className="text-action" onClick={() => void Desktop.OpenURL(`https://github.com/${item.repository}`).catch((failure) => setItemErrors((current) => ({ ...current, [item.id]: errorText(failure) })))}>{item.repository}</button> : '未绑定来源仓库'}{item.sourcePath && item.sourcePath !== '.' && <span> / {item.sourcePath}</span>}</div>
            {expanded.has(item.id) && <div className="skill-details"><div className="skill-full-path">{item.path}</div>{item.revision && <span>安装版本 {item.revision.slice(0, 8)}</span>}{!!item.skills?.length && <ul>{item.skills?.map((skill) => <li key={skill.path}><span>{skill.name}</span><code>{skill.path === '.' ? 'SKILL.md' : `${skill.path}/SKILL.md`}</code></li>)}</ul>}</div>}
            {(itemErrors[item.id] || item.issue) && <p className="skill-error" role="alert">{itemErrors[item.id] || item.issue}</p>}
          </article>)}
      </div>
    </div>
    {downloadLocation && <DownloadSkills location={downloadLocation} onClose={() => setDownloadLocation(null)} onChanged={() => void refresh(downloadLocation.id)} onNotice={(message) => setNotice({ locationID: downloadLocation.id, message })}/>}
    <Dialog open={deleteItem !== null} onOpenChange={(open) => { if (!open && deleteItem && !busy.has(deleteItem.id)) setDeleteItem(null) }}>
      <DialogContent className="skills-dialog skills-confirm" showCloseButton={!deleteItem || !busy.has(deleteItem.id)} onEscapeKeyDown={(event) => { if (deleteItem && busy.has(deleteItem.id)) event.preventDefault() }}>
        <DialogTitle>删除 {deleteItem?.name}</DialogTitle>
        <DialogDescription>{deleteItem?.linked ? '只删除链接，保留链接指向的原始目录。' : deleteItem?.missing ? '目录已不存在，将移除保存的仓库绑定。' : '将删除此安装目录及其中全部文件，并移除仓库绑定。'}</DialogDescription>
        <div className="skill-full-path">{deleteItem?.path}</div>
        {deleteItem && itemErrors[deleteItem.id] && <p className="form-error" role="alert">{itemErrors[deleteItem.id]}</p>}
        <div className="dialog-footer"><Button variant="outline" disabled={!!deleteItem && busy.has(deleteItem.id)} onClick={() => setDeleteItem(null)}>取消</Button><Button variant="destructive" disabled={!deleteItem || busy.has(deleteItem.id)} onClick={() => deleteItem && void remove(deleteItem)}>{deleteItem && busy.has(deleteItem.id) && <LoaderCircle size={13} className="animate-spin"/>}删除</Button></div>
      </DialogContent>
    </Dialog>
    <Dialog open={overwrite !== null} onOpenChange={(open) => { if (!open && overwrite && !busy.has(overwrite.item.id)) setOverwrite(null) }}>
      <DialogContent className="skills-dialog skills-confirm" showCloseButton={!overwrite || !busy.has(overwrite.item.id)} onEscapeKeyDown={(event) => { if (overwrite && busy.has(overwrite.item.id)) event.preventDefault() }}>
        <DialogTitle>覆盖本地改动？</DialogTitle>
        <DialogDescription>{overwrite?.item.name} 中这些文件发生了本地改动，更新将用仓库最新内容替换整个安装目录。</DialogDescription>
        <div className="skill-full-path">{overwrite?.item.path}</div>
        <ul className="skill-changes">{overwrite?.result.changes?.map((path) => <li key={path}>{path}</li>)}</ul>
        {overwrite && itemErrors[overwrite.item.id] && <p className="form-error" role="alert">{itemErrors[overwrite.item.id]}</p>}
        <div className="dialog-footer"><Button variant="outline" disabled={!!overwrite && busy.has(overwrite.item.id)} onClick={() => setOverwrite(null)}>保留本地内容</Button><Button variant="destructive" disabled={!overwrite || busy.has(overwrite.item.id)} onClick={() => overwrite && void update(overwrite.item, overwrite.result.confirmation)}>{overwrite && busy.has(overwrite.item.id) && <LoaderCircle size={13} className="animate-spin"/>}覆盖并更新</Button></div>
      </DialogContent>
    </Dialog>
  </div>
}

function DownloadSkills({ location, onClose, onChanged, onNotice }: { location: SkillLocation; onClose: () => void; onChanged: () => void; onNotice: (message: string) => void }) {
  const [repository, setRepository] = useState('')
  const [extract, setExtract] = useState(true)
  const [preview, setPreview] = useState<SkillPreview | null>(null)
  const [selected, setSelected] = useState<string[]>([])
  const [results, setResults] = useState<SkillInstallResult[]>([])
  const [busy, setBusy] = useState<'preview' | 'install' | 'reset' | ''>('')
  const [error, setError] = useState('')
  const previewRef = useRef('')
  useEffect(() => () => {
    if (previewRef.current) void Backend.DiscardSkillPreview(previewRef.current).catch((failure) => console.error('清理 Skills 下载预览失败', failure))
  }, [])
  const prepare = async () => {
    if (busy) return
    setBusy('preview'); setError('')
    try {
      const next = await Backend.PrepareSkillInstall(repository, extract)
      previewRef.current = next.id
      setPreview(next)
      setSelected(next.mode === 'repository' ? ['.'] : [])
      setResults([])
    } catch (failure) { setError(errorText(failure)) }
    finally { setBusy('') }
  }
  const install = async () => {
    if (!preview || busy || selected.length === 0) return
    setBusy('install'); setError('')
    try {
      const next = (await Backend.InstallSkills(preview.id, location.id, selected)) ?? []
      setResults((current) => [...current.filter((item) => !next.some((result) => result.sourcePath === item.sourcePath)), ...next])
      const successful = next.filter((item) => !item.error)
      if (successful.length) {
        onChanged()
        onNotice([`已安装 ${successful.length} 项`, ...successful.map((item) => item.warning).filter(Boolean)].join('；'))
      }
      if (next.every((item) => !item.error)) onClose()
      else { setSelected(next.filter((item) => item.error).map((item) => item.sourcePath)); setError('部分项目安装失败，已成功的项目保留，可修改选择后重试。') }
    } catch (failure) { setError(errorText(failure)) }
    finally { setBusy('') }
  }
  const reset = async () => {
    if (!preview || busy) return
    setBusy('reset')
    try {
      await Backend.DiscardSkillPreview(preview.id)
      previewRef.current = ''; setPreview(null); setSelected([]); setResults([]); setError('')
    } catch (failure) { setError(errorText(failure)) }
    finally { setBusy('') }
  }
  const completed = new Set(results.filter((item) => !item.error).map((item) => item.sourcePath))
  const available = preview?.candidates?.filter((item) => !completed.has(item.path)) ?? []
  return <Dialog open onOpenChange={(open) => { if (!open && !busy) onClose() }}>
    <DialogContent className="skills-dialog" showCloseButton={!busy} onEscapeKeyDown={(event) => { if (busy) event.preventDefault() }}>
      <DialogTitle>下载 Skills</DialogTitle>
      <DialogDescription className="skill-full-path">安装到 {location.path}</DialogDescription>
      <form className="skills-download-form" onSubmit={(event) => { event.preventDefault(); void (preview ? install() : prepare()) }}>
        <div className="skills-dialog-body">
          <div className="field"><label htmlFor="skill-repository">GitHub 仓库</label><Input id="skill-repository" autoFocus placeholder="https://github.com/owner/repo" value={repository} disabled={!!busy || !!preview} onChange={(event) => setRepository(event.target.value)} required/></div>
          <label className="skills-checkbox"><input type="checkbox" checked={extract} disabled={!!busy || !!preview} onChange={(event) => setExtract(event.target.checked)}/>自动提取 skills</label>
          <p className="skills-help">{extract ? '识别后勾选所需技能，完整复制选中的技能文件夹。' : '以仓库名保留完整目录，整体更新和删除。'}</p>
          {extract && <p className="skills-help">技能依赖仓库共享文件时，请取消勾选以保留完整结构。</p>}
          {preview && <>
            <div className="skills-preview-heading"><strong>{preview.repository}</strong><span>{preview.mode === 'repository' ? '整仓库安装' : `${preview.candidates?.length ?? 0} 个候选技能`}</span><button type="button" className="text-action" disabled={!!busy} onClick={() => void reset()}>重新选择</button></div>
            {preview.mode === 'extracted' && <label className="skills-checkbox"><input type="checkbox" disabled={!!busy || !available.length} checked={available.length > 0 && selected.length === available.length} onChange={(event) => setSelected(event.target.checked ? available.map((item) => item.path) : [])}/>全选</label>}
            <div className="skills-candidates">
              {preview.candidates?.map((item) => { const result = results.find((result) => result.sourcePath === item.path); return <div className="skill-candidate" key={item.path}>
                <label className="skills-checkbox"><input type="checkbox" checked={completed.has(item.path) || selected.includes(item.path)} disabled={!!busy || completed.has(item.path)} onChange={(event) => setSelected((current) => event.target.checked ? [...current, item.path] : current.filter((path) => path !== item.path))}/><span><strong>{item.name}</strong><code>{item.path === '.' ? '仓库根目录' : item.path}</code></span>{completed.has(item.path) && <Check size={14} className="success"/>}</label>
                {result?.error && <p className="skill-error" role="alert">{result.error}</p>}
              </div> })}
            </div>
          </>}
          {error && <p className="form-error skills-download-error" role="alert"><AlertCircle size={13}/>{error}</p>}
        </div>
        <div className="dialog-footer"><Button type="button" variant="outline" disabled={!!busy} onClick={onClose}>取消</Button><Button type="submit" disabled={!!busy || (!preview && !repository.trim()) || (!!preview && !selected.length)}>{busy && <LoaderCircle size={13} className="animate-spin"/>}{busy === 'preview' ? '下载并识别中…' : busy === 'install' ? '安装中…' : busy === 'reset' ? '正在清理预览…' : preview ? `安装 ${selected.length} 项` : '下载并识别'}</Button></div>
      </form>
    </DialogContent>
  </Dialog>
}
