import { useCallback, useEffect, useRef, useState, useSyncExternalStore } from 'react'
import { ChevronDown, ChevronRight, Download, FolderOpen, Link2, LoaderCircle, MoreHorizontal, Plus, RefreshCw, Trash2 } from 'lucide-react'
import * as Backend from '../../../bindings/trylet/internal/skills/service'
import * as Desktop from '../../../bindings/trylet/internal/desktop/desktop'
import type { SkillInstallation, SkillLocation, SkillUpdateResult, SkillWorkspace } from '../../../bindings/trylet/internal/skills/models'
import type { SkillPreferences } from '../../../bindings/trylet/internal/store/models'
import { Button } from '../../components/ui/button'
import { Input } from '../../components/ui/input'
import InstallSkillsDialog from './InstallSkillsDialog'
import { errorText } from '../../lib/language'
import './skills.css'
import { SkillBatchTask, reconcileSelection, updateUnavailable } from './batchTask'
import BatchTaskDialog from './BatchTaskDialog'
import { useFeedback } from '../../components/feedback/FeedbackProvider'
import { ConfirmDialog } from '../../components/feedback/ConfirmDialog'
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem } from '../../components/ui/dropdown-menu'


const toolNames: Record<string, string> = { agents: '共享 · .agents', codex: 'Codex · .codex', claude: 'Claude Code · .claude', custom: '自定义' }

export default function SkillsPanel({ active }: { active: boolean }) {
  const { notify } = useFeedback()
  const [batchMode, setBatchMode] = useState(false)
  const [selected, setSelected] = useState<string[]>([])
  const [deleteTargets, setDeleteTargets] = useState<SkillInstallation[]>([])
  const [task] = useState(() => new SkillBatchTask({ update: Backend.UpdateSkill, remove: Backend.DeleteSkill }))
  const batch = useSyncExternalStore(task.subscribe, task.getSnapshot)
  const batchBusy = batch.phase === 'running' || batch.phase === 'confirmation'

  const contextLock = useRef(false)
  const directoryLock = useRef(false)
  const operationLocks = useRef(new Set<string>())
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
    if (!workspace || contextLock.current || batchBusy) return
    contextLock.current = true
    setContextBusy(true)
    setWorkspaceError('')
    try { setWorkspace(await Backend.SaveSkillPreferences({ ...workspace.preferences, ...patch })) }
    catch (failure) { notify(errorText(failure), 'error') }
    finally { contextLock.current = false; setContextBusy(false) }
  }
  const chooseDirectory = async (custom: boolean) => {
    if (!workspace || directoryLock.current || batchBusy) return
    directoryLock.current = true
    try {
      const path = await Desktop.ChooseSkillsDirectory()
      if (!path) return
      if (custom) {
        const project = workspace.preferences.scope === 'project' ? workspace.preferences.project : ''
        await changeContext({ location: path, customLocations: [...(workspace.preferences.customLocations ?? []).filter((item) => !(item.path.toLowerCase() === path.toLowerCase() && item.project.toLowerCase() === project.toLowerCase())), { path, project }] })
      } else await changeContext({ scope: 'project', project: path, location: '' })
    } catch (failure) { notify(errorText(failure), 'error') }
    finally { directoryLock.current = false }
  }
  const markBusy = (id: string, value: boolean) => setBusy((current) => {
    const next = new Set(current)
    if (value) next.add(id); else next.delete(id)
    return next
  })
  const update = async (item: SkillInstallation, confirmation = '') => {
    if (operationLocks.current.size > 0 || batchBusy) return
    operationLocks.current.add(item.id)
    markBusy(item.id, true)
    setItemErrors((current) => ({ ...current, [item.id]: '' }))
    try {
      const result = await Backend.UpdateSkill(item.id, confirmation)
      if (result.needsConfirmation) setOverwrite({ item, result })
      else {
        setOverwrite(null)
        notify(`${item.name}：${result.warning || result.message}`, result.warning ? 'warning' : 'success')
        await refresh(item.locationID)
      }
    } catch (failure) {
      setItemErrors((current) => ({ ...current, [item.id]: errorText(failure) }))
      if (!confirmation) notify(`${item.name}：更新失败`, 'error')
    }
    finally { operationLocks.current.delete(item.id); markBusy(item.id, false) }
  }
  const remove = async (item: SkillInstallation) => {
    if (operationLocks.current.size > 0 || batchBusy) return
    operationLocks.current.add(item.id)
    markBusy(item.id, true)
    setItemErrors((current) => ({ ...current, [item.id]: '' }))
    try {
      const warning = await Backend.DeleteSkill(item.locationID, item.name)
      setDeleteItem(null)
      notify(warning || `已删除 ${item.name}`, warning ? 'warning' : 'success')
      await refresh(item.locationID)
    } catch (failure) { setItemErrors((current) => ({ ...current, [item.id]: errorText(failure) })) }
    finally { operationLocks.current.delete(item.id); markBusy(item.id, false) }
  }
  const items = snapshot.locationID === location?.id ? snapshot.items : []
  useEffect(() => { setSelected([]) }, [query, location?.id, workspace?.preferences.scope])
  useEffect(() => { setSelected((ids) => reconcileSelection(ids, snapshot.items)) }, [snapshot])
  useEffect(() => { if (batch.phase === 'done') { setSelected([]); if (locationRef.current) void refresh(locationRef.current) } }, [batch.phase, refresh])
  const selectedItems = items.filter((item) => selected.includes(item.id) && !item.readOnly)
  const updateCount = selectedItems.filter((item) => !updateUnavailable(item)).length
  const search = query.trim().toLowerCase()
  const filtered = items.filter((item) => `${item.name} ${item.repository} ${(item.skills ?? []).map((skill) => `${skill.name} ${skill.path}`).join(' ')}`.toLowerCase().includes(search))

  const selectable = filtered.filter((item) => !item.readOnly)
  const allSelected = selectable.length > 0 && selectable.every((item) => selected.includes(item.id))
  if (!workspace) return <div className="panel-loading">{workspaceError ? <><span role="alert">{workspaceError}</span><Button variant="outline" onClick={() => void boot()}>重试</Button></> : <><LoaderCircle size={18} className="animate-spin"/>正在读取 Skills 位置…</>}</div>
  return <div className="skills-inline" data-operation-busy={busy.size > 0 || batchBusy}>
    <header className="page-toolbar skills-toolbar">
      <nav className="section-tabs" aria-label="Skills 作用域">
        <button type="button" aria-pressed={workspace.preferences.scope === 'user'} disabled={contextBusy || busy.size > 0 || batchBusy} onClick={() => void changeContext({ scope: 'user', location: '' })}>用户级</button>
        <button type="button" aria-pressed={workspace.preferences.scope === 'project'} disabled={contextBusy || busy.size > 0 || batchBusy} onClick={() => void changeContext({ scope: 'project', location: '' })}>项目级</button>
      </nav>
      <div className="top-spacer"/>
      {workspace.preferences.scope === 'project' && <Button variant="outline" size="sm" disabled={contextBusy || busy.size > 0 || batchBusy} onClick={() => void chooseDirectory(false)}><FolderOpen size={13}/>{workspace.preferences.project ? '更换项目' : '选择项目'}</Button>}
      {!batchMode && <Button size="sm" variant="outline" disabled={!location || busy.size > 0 || batchBusy} onClick={() => setBatchMode(true)}>批量管理</Button>}
      {!batchMode && <Button size="sm" disabled={!location || location.readOnly || contextBusy || busy.size > 0 || batchBusy} title={location?.readOnly ? '系统内置目录只读' : '从 GitHub 安装 Skills'} onClick={() => setDownloadLocation(location!)}><Download size={13}/>安装 Skill</Button>}
    </header>
    <div className="skills-location-bar">
      <label htmlFor="skills-location">安装位置</label>
      <select id="skills-location" className="native-select" value={location?.path ?? ''} disabled={contextBusy || busy.size > 0 || batchBusy || !location} onChange={(event) => void changeContext({ location: event.target.value })}>
        {!location && <option value="">请先选择项目</option>}
        {workspace.locations?.filter((item) => item.scope === workspace.preferences.scope).map((item) => <option key={item.id} value={item.path}>{toolNames[item.tool]}{item.tool === 'custom' ? ` · ${item.path}` : ''}</option>)}
      </select>
      <DropdownMenu><DropdownMenuTrigger className="icon-button" aria-label="管理安装位置" disabled={contextBusy || busy.size > 0 || batchBusy}><MoreHorizontal size={15}/></DropdownMenuTrigger><DropdownMenuContent align="end"><DropdownMenuItem disabled={workspace.preferences.scope === 'project' && !workspace.preferences.project} onSelect={() => void chooseDirectory(true)}><Plus size={14}/>添加自定义目录</DropdownMenuItem></DropdownMenuContent></DropdownMenu>
    </div>
    {location && <div className="skills-location-path" title={location.path}>{location.path}</div>}
    <div className="skills-workspace">
      <div className="skills-search"><Input aria-label="搜索技能名称或仓库" placeholder="搜索技能名称或仓库" value={query} onChange={(event) => setQuery(event.target.value)}/><span>{items.length} 个安装项</span><button className="icon-button" type="button" title="刷新技能列表" aria-label="刷新技能列表" disabled={!location || loading || contextBusy || busy.size > 0 || batchBusy} onClick={() => location && void refresh(location.id)}><RefreshCw size={14} className={loading ? 'animate-spin' : ''}/></button></div>
      {batchMode && <div className="skills-bulk-bar">
        <label><input type="checkbox" aria-label="全选当前结果" checked={allSelected} ref={(node) => { if (node) node.indeterminate = !allSelected && selected.length > 0 }} disabled={!selectable.length || batchBusy} onChange={(event) => setSelected(event.target.checked ? selectable.map((item) => item.id) : [])}/>全选</label>
        <span>已选 {selectedItems.length} 项</span>
        <Button size="sm" variant="outline" disabled={!updateCount || batchBusy} onClick={() => task.start('update', selectedItems)}>更新所选（{updateCount}）</Button>
        <Button size="sm" variant="outline" disabled={!selectedItems.length || batchBusy} onClick={() => setDeleteTargets(structuredClone(selectedItems))}>删除所选</Button>
        <Button size="sm" variant="ghost" disabled={batchBusy} onClick={() => { setBatchMode(false); setSelected([]) }}>退出批量管理</Button>
      </div>}
      {loadError && <div className="skills-feedback form-error" role="alert">{loadError} <button type="button" className="text-action" onClick={() => location && void refresh(location.id)}>重试</button></div>}
      <div className="skills-list" aria-busy={loading}>
        {!location ? <div className="skills-empty"><FolderOpen size={22}/><strong>选择一个项目目录</strong><span>查看或安装项目使用的 Skills。</span><Button variant="outline" onClick={() => void chooseDirectory(false)}>选择项目</Button></div>
          : loading && items.length === 0 ? <div className="skills-empty"><LoaderCircle size={20} className="animate-spin"/>正在扫描 Skills…</div>
          : !loadError && filtered.length === 0 ? <div className="skills-empty"><strong>{search ? '没有匹配的技能' : '此位置还没有 Skills'}</strong><span>{search ? '换个名称或仓库试试。' : '从 GitHub 安装技能，或切换安装位置。'}</span>{search && <Button variant="outline" onClick={() => setQuery('')}>清除搜索</Button>}</div>
          : filtered.map((item) => <article className="skill-row" key={item.id}>
            <div className="skill-row-heading">
              {batchMode && <input type="checkbox" aria-label={`选择 ${item.name}`} disabled={item.readOnly || batchBusy} checked={selected.includes(item.id)} onChange={(event) => setSelected((ids) => event.target.checked ? [...ids, item.id] : ids.filter((id) => id !== item.id))}/>}
              <button className="skill-name" type="button" aria-expanded={expanded.has(item.id)} onClick={() => setExpanded((current) => { const next = new Set(current); if (next.has(item.id)) next.delete(item.id); else next.add(item.id); return next })}>
                {expanded.has(item.id) ? <ChevronDown size={13}/> : <ChevronRight size={13}/>}<strong>{item.name}</strong>{item.linked && <Link2 size={13} aria-label="链接目录"/>}
              </button>
              <span className="skill-state">{item.readOnly ? '系统内置 · 只读' : item.linked ? '链接' : item.missing ? '目录缺失' : item.mode === 'git' ? `Git 克隆 · 包含 ${item.skills?.length ?? 0} 个技能` : item.mode === 'repository' ? `整仓库 · 包含 ${item.skills?.length ?? 0} 个技能` : `${item.skills?.length ?? 0} 个技能`}</span>
              <div className="skill-actions" hidden={batchMode}>
                {!item.readOnly && <><Button variant="ghost" size="xs" disabled={!item.repository || item.linked || (item.mode === 'git' && item.missing) || busy.size > 0 || batchBusy} title={item.mode === 'git' ? '快进更新当前 Git 分支，保留本地改动' : item.repository ? '从来源仓库更新' : '未绑定仓库，无法更新'} onClick={() => void update(item)}>{busy.has(item.id) ? <LoaderCircle size={12} className="animate-spin"/> : <RefreshCw size={12}/>}更新</Button>
                  <DropdownMenu><DropdownMenuTrigger className="icon-button" aria-label={`${item.name} 的更多操作`} disabled={busy.size > 0 || batchBusy}><MoreHorizontal size={15}/></DropdownMenuTrigger><DropdownMenuContent align="end"><DropdownMenuItem variant="destructive" onSelect={() => setDeleteItem(item)}><Trash2 size={13}/>{item.linked ? '移除链接' : item.missing ? '移除记录' : '删除安装'}</DropdownMenuItem></DropdownMenuContent></DropdownMenu></>}
              </div>
            </div>
            <div className="skill-source">{item.repository ? <button type="button" className="text-action" onClick={() => void Desktop.OpenURL(`https://github.com/${item.repository}`).catch((failure) => notify(errorText(failure), 'error'))}>{item.repository}</button> : '未绑定来源仓库'}{item.sourcePath && item.sourcePath !== '.' && <span> / {item.sourcePath}</span>}</div>
            {expanded.has(item.id) && <div className="skill-details"><div className="skill-full-path">{item.path}</div>{item.revision && <span>安装版本 {item.revision.slice(0, 8)}</span>}{!!item.skills?.length && <ul>{item.skills?.map((skill) => <li key={skill.path}><span>{skill.name}</span><code>{skill.path === '.' ? 'SKILL.md' : `${skill.path}/SKILL.md`}</code></li>)}</ul>}</div>}
            {batchMode && updateUnavailable(item) && <p className="skills-help">不可更新：{updateUnavailable(item)}</p>}
            {itemErrors[item.id] && <details className="skill-issue"><summary>上次操作失败</summary><p>{itemErrors[item.id]}</p></details>}
            {item.issue && <details className="skill-issue"><summary>此安装项需要处理</summary><p>{item.issue}</p></details>}
          </article>)}
      </div>
    </div>
    <BatchTaskDialog task={task} state={batch}/>
    <ConfirmDialog open={deleteTargets.length > 0} title={`删除所选 ${deleteTargets.length} 个安装项？`}
      description={`删除目录 ${deleteTargets.filter((item) => !item.linked && !item.missing).length} 项；仅移除链接 ${deleteTargets.filter((item) => item.linked).length} 项；仅移除记录 ${deleteTargets.filter((item) => !item.linked && item.missing).length} 项。删除目录及其中的文件无法撤销。`}
      destructive confirmLabel={`删除所选（${deleteTargets.length}）`} onCancel={() => setDeleteTargets([])} onConfirm={() => { const targets = deleteTargets; setDeleteTargets([]); task.start('delete', targets) }}>
      <ul className="batch-results">{deleteTargets.map((item) => <li key={item.id}><strong>{item.name}</strong><p>{item.path}</p></li>)}</ul>
    </ConfirmDialog>
    {downloadLocation && <InstallSkillsDialog location={downloadLocation} onClose={() => setDownloadLocation(null)} onChanged={() => void refresh(downloadLocation.id)}/>}
    <ConfirmDialog open={!!deleteItem} title={`${deleteItem?.linked ? '移除链接' : deleteItem?.missing ? '移除记录' : '删除安装'}：${deleteItem?.name ?? ''}`}
      description={deleteItem?.linked ? '只删除链接，原始目录及文件保留。' : deleteItem?.missing ? '目录已不存在，将移除保存的仓库绑定。' : '将删除此安装目录及全部文件，并移除仓库绑定。此操作无法撤销。'}
      busy={!!deleteItem && busy.has(deleteItem.id)} error={deleteItem ? itemErrors[deleteItem.id] : ''}
      confirmLabel={deleteItem?.linked ? '移除链接' : deleteItem?.missing ? '移除记录' : '删除安装'} destructive
      onCancel={() => setDeleteItem(null)} onConfirm={() => { if (deleteItem) void remove(deleteItem) }}>
      <div className="skill-full-path">{deleteItem?.path}</div>
    </ConfirmDialog>
    <ConfirmDialog open={!!overwrite} title="覆盖本地改动？"
      description={`${overwrite?.item.name ?? ''} 存在本地改动，更新将替换整个安装目录。请先备份需要保留的文件。`}
      busy={!!overwrite && busy.has(overwrite.item.id)} error={overwrite ? itemErrors[overwrite.item.id] : ''}
      confirmLabel="覆盖并更新" cancelLabel="保留本地内容" destructive
      onCancel={() => setOverwrite(null)} onConfirm={() => { if (overwrite) void update(overwrite.item, overwrite.result.confirmation) }}>
      <div className="skill-full-path">{overwrite?.item.path}</div>
      <ul className="skill-changes">{overwrite?.result.changes?.map((path) => <li key={path}>{path}</li>)}</ul>
    </ConfirmDialog>
  </div>
}
