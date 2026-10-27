import { useEffect, useRef, useState } from 'react'
import { ExternalLink, LoaderCircle, RefreshCw } from 'lucide-react'
import { Button } from './components/ui/button'
import { Input } from './components/ui/input'
import * as Backend from '../bindings/trylet/internal/trylet/service'
import * as Desktop from '../bindings/trylet/desktop'
import type { RuntimeCandidate, Settings } from '../bindings/trylet/internal/trylet/models'
import { errorText, languageNames } from './lib/language'

type Props = {
  active: boolean
  settings: Settings
  tokenSet: boolean
  runtimes: RuntimeCandidate[]
  detecting: boolean
  onDetect: () => Promise<void>
  onClose: () => void
  onSaved: (settings: Settings, tokenSet: boolean) => void
  onTokenRemoved: () => void
}

export default function SettingsPanel(props: Props) {
  const [form, setForm] = useState<Settings>(() => structuredClone(props.settings))
  const [token, setToken] = useState('')
  const [hasToken, setHasToken] = useState(props.tokenSet)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [removing, setRemoving] = useState(false)
  const repository = form.repository
    .trim()
    .replace(/^https:\/\/github\.com\//, '')
    .replace(/\.git\/?$/, '')
    .replace(/\/$/, '')
  const createToken = () => {
    const url = new URL('https://github.com/settings/personal-access-tokens/new')
    url.searchParams.set('contents', 'write')
    const owner = /^([a-zA-Z0-9-]+)\/[^/]+$/.exec(repository)?.[1]
    if (owner) url.searchParams.set('target_name', owner)
    void Desktop.OpenURL(url.toString()).catch((failure) => setError(errorText(failure)))
  }
  const save = async () => {
    setSaving(true)
    setError('')
    try {
      const value: Settings = {
        ...form,
        repository,
        branch: form.branch.trim(),
        shortcut: form.shortcut.trim(),
        customPaths: Object.fromEntries(
          Object.entries(form.customPaths ?? {}).map(([key, path]) => [
            key,
            (path ?? '').trim().replace(/^"(.*)"$/, '$1'),
          ]),
        ),
      }
      const tokenSet = await Backend.SavePreferences(value, token)
      setToken('')
      setHasToken(tokenSet)
      setForm(value)
      props.onSaved(value, tokenSet)
    } catch (failure) {
      setError(errorText(failure))
    } finally {
      setSaving(false)
    }
  }
  const saveRef = useRef(save)
  saveRef.current = save
  useEffect(() => {
    const key = (event: KeyboardEvent) => {
      if (props.active && !event.isComposing && event.ctrlKey && event.key.toLowerCase() === 's') {
        event.preventDefault()
        if (!saving && !removing) void saveRef.current()
      }
    }
    window.addEventListener('keydown',key)
    return () => window.removeEventListener('keydown',key)
  },[props.active,saving,removing])
  return (
    <section className="settings-inline" aria-label="设置">
        <div className="dialog-heading">
          <h2>设置</h2>
          <p>使用这台电脑已有的环境，保持练习简单。</p>
        </div>
        <div className="dialog-scroll">
          <div className="section-label">编辑与窗口</div>
          <label className="check-field">
            <input
              type="checkbox"
              checked={form.autoTemplate}
              onChange={(e) => setForm({ ...form, autoTemplate: e.target.checked })}
            />
            空白练习选择语言时，自动填入基础模板
          </label>
          <div className="field" style={{ marginTop: 16 }}>
            <label htmlFor="global-shortcut">全局唤出快捷键</label>
            <Input
              id="global-shortcut"
              value={form.shortcut}
              onChange={(e) => setForm({ ...form, shortcut: e.target.value })}
              placeholder="Ctrl+Alt+Space"
            />
            <small>留空可关闭；若已被其他程序占用，会保留原来的快捷键。</small>
          </div>
          <div className="settings-divider" />
          <div className="section-label">
            <span>本机运行环境</span>
            <Button
              variant="ghost"
              size="sm"
              disabled={props.detecting || saving}
              onClick={() => void props.onDetect()}
            >
              {props.detecting ? (
                <LoaderCircle size={12} className="animate-spin" />
              ) : (
                <RefreshCw size={12} />
              )}
              重新检测
            </Button>
          </div>
          {(['c', 'cpp', 'python'] as const).map((language) => {
            const candidates = props.runtimes.filter((item) => item.language === language)
            const selected =
              candidates.find((item) => item.id === form.runtimes?.[language]) ??
              candidates.find((item) => item.kind === 'gcc') ??
              candidates[0]
            return (
              <div className="runtime-row" key={language}>
                <div className="field-label">{languageNames[language]}</div>
                <div className="field">
                  <select
                    className="native-select"
                    aria-label={`${languageNames[language]} 运行环境`}
                    value={form.runtimes?.[language] ?? ''}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        runtimes: { ...form.runtimes, [language]: e.target.value },
                      })
                    }
                  >
                    <option value="">
                      自动选择{!candidates.length ? ' · 未检测到可用环境' : ''}
                    </option>
                    {candidates.map((candidate) => (
                      <option key={candidate.id} value={candidate.id}>
                        {candidate.name} · {candidate.version}
                      </option>
                    ))}
                  </select>
                  {selected && <div className="runtime-path">{selected.path}</div>}
                </div>
                <div className="field">
                  <Input
                    aria-label={`${languageNames[language]} 自定义路径`}
                    placeholder={
                      language === 'python'
                        ? '或指定 python.exe 的完整路径'
                        : '或指定编译器 .exe 的完整路径'
                    }
                    value={form.customPaths?.[language] ?? ''}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        customPaths: { ...form.customPaths, [language]: e.target.value },
                      })
                    }
                  />
                </div>
              </div>
            )
          })}
          <p className="help">
            自定义路径保存后重新检测。C/C++ 编译完整单文件；Python 使用对应解释器中已有的依赖。
          </p>
          <div className="settings-divider" />
          <div className="section-label">GitHub 发布</div>
          <div className="field">
            <label htmlFor="repository">默认仓库</label>
            <Input
              id="repository"
              value={form.repository}
              onChange={(e) => setForm({ ...form, repository: e.target.value })}
              placeholder="owner/practise-daily"
            />
          </div>
          <div className="field">
            <label htmlFor="default-branch">默认分支</label>
            <Input
              id="default-branch"
              value={form.branch}
              onChange={(e) => setForm({ ...form, branch: e.target.value })}
              placeholder="留空使用仓库默认分支"
            />
          </div>
          <div className="field">
            <label htmlFor="github-token">{hasToken ? '替换访问令牌' : '仓库访问令牌'}</label>
            <Input
              id="github-token"
              type="password"
              autoComplete="new-password"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder={
                hasToken
                  ? '已加密保存在同级数据文件，留空保留'
                  : '粘贴 fine-grained personal access token'
              }
            />
            <small>
              创建页预填 Contents 读写权限；仍需手动选择具体仓库。令牌加密保存在同级文件，更换电脑或 Windows 账户后需重新填写。
            </small>
          </div>
          <div className="token-state">
            <Button
              variant="link"
              size="sm"
              onClick={createToken}
            >
              创建细粒度令牌
              <ExternalLink size={11} />
            </Button>
            {hasToken && (
              <Button
                variant="ghost"
                size="sm"
                disabled={removing || saving}
                onClick={() => {
                  setRemoving(true)
                  void Backend.RemoveToken()
                    .then(() => {
                      setHasToken(false)
                      props.onTokenRemoved()
                    })
                    .catch((failure) => setError(errorText(failure)))
                    .finally(() => setRemoving(false))
                }}
              >
                {removing ? '移除中…' : '移除已保存令牌'}
              </Button>
            )}
          </div>
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
        </div>
        <div className="dialog-footer">
          <Button variant="outline" disabled={saving || removing} onClick={() => {setForm(structuredClone(props.settings));setToken('');setError('');props.onClose()}}>
            取消
          </Button>
          <Button disabled={saving || removing} onClick={() => void save()}>
            {saving && <LoaderCircle size={13} className="animate-spin" />}
            {saving ? '保存中…' : '保存设置'}
          </Button>
        </div>
    </section>
  )
}
