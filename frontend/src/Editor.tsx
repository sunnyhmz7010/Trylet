import { useEffect, useRef } from 'react'
import { EditorView, basicSetup } from 'codemirror'
import { Compartment, EditorState } from '@codemirror/state'
import { placeholder } from '@codemirror/view'
import { cpp } from '@codemirror/lang-cpp'
import { python } from '@codemirror/lang-python'
import { codeHighlighting, codeTheme } from './codeAppearance'
import { copyHighlightedHTML } from './editorClipboard'

type Props = {
  value: string
  language: string | null
  locked: boolean
  onChange: (value: string) => void
}

export default function Editor({ value, language, locked, onChange }: Props) {
  const mount = useRef<HTMLDivElement>(null)
  const view = useRef<EditorView | null>(null)
  const callback = useRef(onChange)
  callback.current = onChange
  const languageConfig = useRef(new Compartment())
  const readOnlyConfig = useRef(new Compartment())
  const themeConfig = useRef(new Compartment())
  useEffect(() => {
    const instance = new EditorView({
      parent: mount.current!,
      state: EditorState.create({
        doc: value,
        extensions: [
          basicSetup,
          codeHighlighting,
          themeConfig.current.of(codeTheme()),
          placeholder('在这里写几行代码，试试你的想法。'),
          languageConfig.current.of(
            language === 'python' ? python() : language === 'c' || language === 'cpp' ? cpp() : [],
          ),
          readOnlyConfig.current.of([
            EditorState.readOnly.of(locked),
            EditorView.editable.of(!locked),
          ]),
          EditorView.contentAttributes.of({
            'aria-label': '代码编辑器',
            spellcheck: 'false',
            autocorrect: 'off',
            autocapitalize: 'off',
          }),
          EditorView.updateListener.of((update) => {
            if (update.docChanged) callback.current(update.state.doc.toString())
          }),
        ],
      }),
    })
    view.current = instance
    const copy = (event: ClipboardEvent) => copyHighlightedHTML(event, instance)
    instance.dom.addEventListener('copy', copy)
    const theme = () => instance.dispatch({ effects: themeConfig.current.reconfigure(codeTheme()) })
    window.addEventListener('trylet:theme-changed', theme)
    instance.focus()
    return () => {
      window.removeEventListener('trylet:theme-changed', theme)
      instance.dom.removeEventListener('copy', copy)
      instance.destroy()
      view.current = null
    }
  }, [])
  useEffect(() => {
    const instance = view.current
    if (instance && instance.state.doc.toString() !== value)
      instance.dispatch({ changes: { from: 0, to: instance.state.doc.length, insert: value } })
  }, [value])
  useEffect(() => {
    view.current?.dispatch({
      effects: languageConfig.current.reconfigure(
        language === 'python' ? python() : language === 'c' || language === 'cpp' ? cpp() : [],
      ),
    })
  }, [language])
  useEffect(() => {
    view.current?.dispatch({
      effects: readOnlyConfig.current.reconfigure([
        EditorState.readOnly.of(locked),
        EditorView.editable.of(!locked),
      ]),
    })
  }, [locked])
  return <div ref={mount} className="editor-mount" />
}
