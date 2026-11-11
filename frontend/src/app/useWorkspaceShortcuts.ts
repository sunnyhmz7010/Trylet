import { useEffect, useRef } from 'react'
import type { View } from './useWorkspaceActions'

type Shortcuts = {
  activeView: View
  run: () => Promise<void>
  create: () => void
  publish: () => Promise<void>
  save: () => Promise<void>
  hide: () => Promise<void>
}
export function useWorkspaceShortcuts(actions: Shortcuts) {
  const latest = useRef(actions)
  latest.current = actions
  useEffect(() => {
    const key = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.repeat || event.isComposing || document.querySelector('[role="dialog"]')) return
      if (latest.current.activeView !== 'editor') return
      if (event.ctrlKey && event.key === 'Enter') {
        event.preventDefault()
        void latest.current.run()
      } else if (event.ctrlKey && !event.shiftKey && event.key.toLowerCase() === 'n') {
        event.preventDefault()
        latest.current.create()
      } else if (event.ctrlKey && event.shiftKey && event.key.toLowerCase() === 's') {
        event.preventDefault()
        void latest.current.publish()
      } else if (event.ctrlKey && event.key.toLowerCase() === 's') {
        event.preventDefault()
        void latest.current.save()
      } else if (
        event.key === 'Escape' &&
        !document.querySelector('[data-state="open"][role="menu"]') &&
        !(event.target as HTMLElement).closest('.terminal-mount')
      ) {
        event.preventDefault()
        void latest.current.hide()
      }
    }
    window.addEventListener('keydown', key)
    return () => window.removeEventListener('keydown', key)
  }, [])
}
