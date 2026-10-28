import { useEffect, useRef } from 'react'
import { MergeView } from '@codemirror/merge'
import { Compartment, EditorState } from '@codemirror/state'
import { EditorView, lineNumbers } from '@codemirror/view'
import { codeTheme } from './codeAppearance'

export default function Diff({ remote, local }: { remote: string; local: string }) {
  const mount = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const themeConfig = new Compartment()
    const extensions = [
      themeConfig.of(codeTheme()),
      lineNumbers(),
      EditorState.readOnly.of(true),
      EditorView.editable.of(false),
      EditorView.lineWrapping,
    ]
    const view = new MergeView({
      parent: mount.current!,
      a: { doc: remote, extensions },
      b: { doc: local, extensions },
      highlightChanges: true,
      gutter: true,
      collapseUnchanged: { margin: 2, minSize: 5 },
    })
    const theme = () => {
      view.a.dispatch({ effects: themeConfig.reconfigure(codeTheme()) })
      view.b.dispatch({ effects: themeConfig.reconfigure(codeTheme()) })
    }
    window.addEventListener('trylet:theme-changed', theme)
    return () => { window.removeEventListener('trylet:theme-changed', theme); view.destroy() }
  }, [remote, local])
  return <div className="diff-mount" ref={mount} />
}
