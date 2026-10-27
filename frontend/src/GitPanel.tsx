import { useEffect, useState } from 'react'
import { AlertCircle, LoaderCircle, X } from 'lucide-react'
import GitApp from './git/App'
import { LanguageProvider } from './git/i18n'
import { ThemeProvider } from './git/theme'
import { loadPreferences } from './git/preferences'
import './git/styles.css'
import './git/themes.css'

export default function GitPanel({ active }: { active: boolean }) {
  const [ready,setReady] = useState(false)
  const [error,setError] = useState('')
  useEffect(() => {
    let live = true
    void loadPreferences().then(() => {if(live) setReady(true)}).catch((failure) => {if(live){setError(String(failure));setReady(true)}})
    const failed = (event: Event) => setError((event as CustomEvent<string>).detail)
    window.addEventListener('trylet:git-preference-error',failed)
    return () => {live=false;window.removeEventListener('trylet:git-preference-error',failed)}
  },[])
  return <div className="git-tab-content">
    {error&&<div className="notice" role="alert"><AlertCircle size={14}/><span>{error}</span><button className="icon-button" aria-label="关闭提示" onClick={()=>setError('')}><X size={13}/></button></div>}
    {ready?<LanguageProvider><ThemeProvider><GitApp active={active}/></ThemeProvider></LanguageProvider>:<div className="panel-loading"><LoaderCircle size={18} className="animate-spin"/>正在加载 Git 视图…</div>}
  </div>
}
