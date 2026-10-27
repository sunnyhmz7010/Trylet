import { useEffect, useRef } from 'react'
import { MergeView } from '@codemirror/merge'
import { EditorState } from '@codemirror/state'
import { EditorView, lineNumbers } from '@codemirror/view'

export default function Diff({ remote, local }: { remote: string; local: string }) {
  const mount = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const extensions = [
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
    return () => view.destroy()
  }, [remote, local])
  return <div className="diff-mount" ref={mount} />
}
