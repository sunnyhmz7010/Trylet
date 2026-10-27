import { lazy, Suspense, useEffect, useRef, useState } from 'react'
import {
  Check,
  ChevronRight,
  ExternalLink,
  Folder,
  FolderPlus,
  Github,
  LoaderCircle,
} from 'lucide-react'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from './components/ui/dialog'
import { Button } from './components/ui/button'
import { Input } from './components/ui/input'
import * as Backend from '../bindings/trylet/internal/trylet/service'
import * as Desktop from '../bindings/trylet/desktop'
import type {
  Directory,
  Draft,
  PublishRecord,
  PublishResult,
  RepositoryInfo,
  Settings,
} from '../bindings/trylet/internal/trylet/models'
import { errorText, extensions, localDate, timeSuffix, type Language } from './lib/language'

const Diff = lazy(() => import('./Diff'))
type Props = {
  draft: Draft
  settings: Settings
  onClose: () => void
  onPublished: (record: PublishRecord) => void
}

export default function PublishDialog({ draft, settings, onClose, onPublished }: Props) {
  const previous = draft.publish?.target
  const [repository, setRepository] = useState(previous?.repository ?? settings.repository)
  const [branch, setBranch] = useState(previous?.branch ?? settings.branch)
  const [directory, setDirectory] = useState(
    previous ? previous.path.split('/').slice(0, -1).join('/') : localDate(),
  )
  const [filename, setFilename] = useState(
    previous?.path.split('/').at(-1) ??
      `practice-${timeSuffix()}${extensions[draft.language as Language]}`,
  )
  const [message, setMessage] = useState(`feat(practice): 添加 ${filename}`)
  const messageEdited = useRef(false)
  const [newFolder, setNewFolder] = useState('')
  const [repositoryInfo, setRepositoryInfo] = useState<
    (RepositoryInfo & { repository: string }) | null
  >(null)
  const [dirInfo, setDirInfo] = useState<Directory | null>(null)
  const [repositoryLoading, setRepositoryLoading] = useState(true)
  const [directoryLoading, setDirectoryLoading] = useState(true)
  const [repositoryError, setRepositoryError] = useState('')
  const [directoryError, setDirectoryError] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [conflict, setConflict] = useState<PublishResult | null>(null)
  const [result, setResult] = useState<PublishResult | null>(null)
  const nameInput = useRef<HTMLInputElement>(null)
  const path = [directory, filename].filter(Boolean).join('/')

  useEffect(() => {
    let live = true
    setRepositoryLoading(true)
    setRepositoryError('')
    setDirInfo(null)
    const timer = setTimeout(() => {
      if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository)) {
        setRepositoryError('请填写 owner/repo 格式的仓库名称')
        setRepositoryLoading(false)
        return
      }
      void Backend.Repository(repository)
        .then((info) => {
          if (!live) return
          setRepositoryInfo({ ...info, repository })
          setBranch((current) => (info.branches?.includes(current) ? current : info.defaultBranch))
        })
        .catch((failure) => {
          if (live) {
            setRepositoryInfo(null)
            setRepositoryError(errorText(failure))
          }
        })
        .finally(() => {
          if (live) setRepositoryLoading(false)
        })
    }, 350)
    return () => {
      live = false
      clearTimeout(timer)
    }
  }, [repository])

  useEffect(() => {
    if (!branch || repositoryInfo?.repository !== repository || repositoryLoading) return
    let live = true
    setDirectoryLoading(true)
    setDirectoryError('')
    setDirInfo(null)
    void Backend.Directory({ repository, branch, path: '' }, directory)
      .then((info) => {
        if (live) setDirInfo(info)
      })
      .catch((failure) => {
        if (live) setDirectoryError(errorText(failure))
      })
      .finally(() => {
        if (live) setDirectoryLoading(false)
      })
    return () => {
      live = false
    }
  }, [repository, repositoryInfo, branch, directory, repositoryLoading])

  useEffect(() => {
    if (!messageEdited.current) setMessage(`feat(practice): 添加 ${filename}`)
  }, [filename])

  const upload = async (confirmed = false, sha = '') => {
    if (busy) return
    if (!filename || /[/\\]/.test(filename)) {
      setError('文件名不能为空，也不能包含目录分隔符。')
      return
    }
    setBusy(true)
    setError('')
    try {
      const response = await Backend.Publish({
        draftID: draft.id,
        code: draft.code,
        language: draft.language,
        target: { repository, branch, path },
        message,
        expectedSHA: sha,
        confirmed,
      })
      if (response.status === 'conflict') setConflict(response)
      else {
        setConflict(null)
        setResult(response)
        if (response.record) onPublished(response.record)
      }
    } catch (failure) {
      setError(errorText(failure))
    } finally {
      setBusy(false)
    }
  }
  const parts = directory ? directory.split('/') : []
  const addFolder = () => {
    const name = newFolder.trim()
    if (!name || /[/\\:\r\n]/.test(name) || name === '.' || name === '..') {
      setDirectoryError('请输入有效的单级目录名。')
      return
    }
    setDirectory([directory, name].filter(Boolean).join('/'))
    setNewFolder('')
  }
  const openURL = (value: string) => {
    void Desktop.OpenURL(value).catch((failure) => setError(errorText(failure)))
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !busy) onClose()
      }}
    >
      <DialogContent className="dialog-content">
        <div className="dialog-heading">
          <DialogTitle>
            {result ? '练习已同步' : conflict ? '目标文件已有不同内容' : '发布到 GitHub'}
          </DialogTitle>
          <DialogDescription>
            {result
              ? result.message
              : conflict
                ? conflict.message
                : '选择保存的位置，把这份练习留在仓库里。'}
          </DialogDescription>
        </div>
        <div className="dialog-scroll">
          {result ? (
            <>
              <div
                className="inline-message"
                style={{ display: 'flex', alignItems: 'center', gap: 7 }}
              >
                <Check size={16} />
                {result.status === 'published' ? '提交成功' : '内容已与远端核对一致'}
              </div>
              <div className="path-preview" style={{ marginTop: 14 }}>
                {repository} · {branch}
                <br />
                <strong>{path}</strong>
              </div>
              <div className="result-links">
                {result.record?.fileURL && (
                  <button onClick={() => openURL(result.record!.fileURL)}>
                    查看文件
                    <ExternalLink size={12} />
                  </button>
                )}
                {result.record?.commitURL && (
                  <button onClick={() => openURL(result.record!.commitURL)}>
                    查看提交
                    <ExternalLink size={12} />
                  </button>
                )}
              </div>
            </>
          ) : conflict ? (
            <>
              <div className="path-preview" style={{ marginBottom: 12 }}>
                {repository} · {branch}
                <br />
                <strong>{path}</strong>
              </div>
              <div className="diff-caption">
                <span>GitHub 当前版本</span>
                <span>本次待发布版本</span>
              </div>
              <Suspense fallback={<p className="help">正在载入差异视图…</p>}>
                <Diff remote={conflict.remoteCode} local={draft.code} />
              </Suspense>
              <p className="help" style={{ marginTop: 10 }}>
                更新原文件会生成新的 commit，历史版本仍保留在 GitHub。
              </p>
            </>
          ) : (
            <>
              <div className="field-row">
                <div className="field" style={{ flex: 2 }}>
                  <label htmlFor="publish-repository">仓库</label>
                  <Input
                    id="publish-repository"
                    value={repository}
                    disabled={busy}
                    onChange={(e) => setRepository(e.target.value.trim())}
                  />
                </div>
                <div className="field">
                  <label htmlFor="publish-branch">分支</label>
                  <select
                    id="publish-branch"
                    className="native-select"
                    value={branch}
                    disabled={busy || repositoryLoading || !repositoryInfo}
                    onChange={(e) => setBranch(e.target.value)}
                  >
                    {repositoryLoading ? (
                      <option value={branch}>读取分支…</option>
                    ) : (
                      repositoryInfo?.branches?.map((item) => (
                        <option key={item} value={item}>
                          {item}
                        </option>
                      ))
                    )}
                  </select>
                </div>
              </div>
              {repositoryError && (
                <p className="form-error" role="alert">
                  {repositoryError}
                </p>
              )}
              <div className="field">
                <label>保存目录</label>
                <div className="directory-box">
                  <div className="breadcrumbs">
                    <button disabled={busy} onClick={() => setDirectory('')}>
                      仓库根目录
                    </button>
                    {parts.map((part, index) => (
                      <span key={index} style={{ display: 'inline-flex', alignItems: 'center' }}>
                        <ChevronRight size={11} />
                        <button
                          disabled={busy}
                          onClick={() => setDirectory(parts.slice(0, index + 1).join('/'))}
                        >
                          {part}
                        </button>
                      </span>
                    ))}
                  </div>
                  <div className="folder-list">
                    {repositoryLoading || directoryLoading ? (
                      <div className="directory-empty">正在读取目录…</div>
                    ) : directoryError ? (
                      <p className="form-error" role="alert">
                        {directoryError}
                      </p>
                    ) : dirInfo?.names?.length ? (
                      dirInfo.names.map((name) => (
                        <button
                          className="folder-item"
                          key={name}
                          disabled={busy}
                          onClick={() => setDirectory([directory, name].filter(Boolean).join('/'))}
                        >
                          <Folder size={14} />
                          {name}
                          <ChevronRight size={12} style={{ marginLeft: 'auto' }} />
                        </button>
                      ))
                    ) : (
                      <div className="directory-empty">
                        {dirInfo?.exists
                          ? '当前目录没有子目录，可直接在这里保存。'
                          : '此目录将在提交文件时一同创建。'}
                      </div>
                    )}
                  </div>
                </div>
                <div className="directory-actions">
                  <Input
                    aria-label="新目录名"
                    value={newFolder}
                    placeholder="新建目录"
                    disabled={busy}
                    onChange={(e) => setNewFolder(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault()
                        addFolder()
                      }
                    }}
                  />
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={busy || !newFolder.trim()}
                    onClick={addFolder}
                  >
                    <FolderPlus size={12} />
                    创建
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={busy}
                    onClick={() => setDirectory(localDate())}
                  >
                    今天
                  </Button>
                </div>
              </div>
              <div className="field">
                <label htmlFor="publish-filename">文件名</label>
                <Input
                  id="publish-filename"
                  ref={nameInput}
                  value={filename}
                  disabled={busy}
                  onChange={(e) => setFilename(e.target.value)}
                />
              </div>
              <div className="field">
                <label htmlFor="publish-message">提交说明</label>
                <Input
                  id="publish-message"
                  value={message}
                  disabled={busy}
                  onChange={(e) => {
                    messageEdited.current = true
                    setMessage(e.target.value)
                  }}
                />
              </div>
              <div className="path-preview">
                最终路径
                <br />
                <strong>
                  {repository}/{path}
                </strong>
              </div>
            </>
          )}
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
        </div>
        <div className="dialog-footer">
          {result ? (
            <Button onClick={onClose}>完成</Button>
          ) : conflict ? (
            <>
              <Button variant="ghost" disabled={busy} onClick={onClose}>
                取消
              </Button>
              <Button
                variant="outline"
                disabled={busy}
                onClick={() => {
                  setConflict(null)
                  setFilename((name) => name.replace(/(\.[^.]+)$/, `-${timeSuffix()}$1`))
                  setTimeout(() => nameInput.current?.focus(), 0)
                }}
              >
                另存为新文件
              </Button>
              <Button disabled={busy} onClick={() => void upload(true, conflict.sha)}>
                {busy && <LoaderCircle size={12} className="animate-spin" />}
                {conflict.sha ? '更新原文件' : '重新创建文件'}
              </Button>
            </>
          ) : (
            <>
              <Button variant="outline" disabled={busy} onClick={onClose}>
                取消
              </Button>
              <Button
                disabled={
                  busy ||
                  repositoryLoading ||
                  !repositoryInfo ||
                  !!repositoryError ||
                  !branch ||
                  !filename.trim() ||
                  !message.trim()
                }
                onClick={() => void upload()}
              >
                {busy ? <LoaderCircle size={13} className="animate-spin" /> : <Github size={13} />}
                {busy ? '正在提交…' : '推送到 GitHub'}
              </Button>
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
