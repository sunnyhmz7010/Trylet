import { useEffect, useState } from 'react'
import { LoaderCircle } from 'lucide-react'
import GitApp from './App'
import { useFeedback } from '../../components/feedback/FeedbackProvider'
import { loadPreferences } from './preferences'
import './styles.css'
import './themes.css'

export default function GitPanel({ active }: { active: boolean }) {
  const [ready,setReady] = useState(false)
  const { notify } = useFeedback()
  useEffect(() => {
    let live = true
    void loadPreferences().then(() => {if(live) setReady(true)}).catch((failure) => {if(live){notify(`读取 Git 偏好失败：${String(failure)}`, 'error');setReady(true)}})
    const failed = (event: Event) => notify(`保存 Git 偏好失败：${(event as CustomEvent<string>).detail}`, 'error')
    window.addEventListener('trylet:git-preference-error',failed)
    return () => {live=false;window.removeEventListener('trylet:git-preference-error',failed)}
  },[notify])
  return <div className="git-tab-content">
    {ready?<GitApp active={active}/>:<div className="panel-loading"><LoaderCircle size={18} className="animate-spin"/>正在加载 Git 视图…</div>}
  </div>
}
