import type { EditorView } from '@codemirror/view'
import { ensureSyntaxTree, highlightingFor, syntaxTree } from '@codemirror/language'
import { highlightTree } from '@lezer/highlight'

// 在 CodeMirror 完成默认复制后补充 HTML，保留其纯文本、多选区和整行复制语义。
export function copyHighlightedHTML(event: ClipboardEvent, view: EditorView) {
  const data = event.clipboardData
  if (!data || !event.defaultPrevented || !view.hasFocus) return
  const text = data.getData('text/plain')
  // 大段复制仍保留纯文本，避免在同步剪贴板事件中长时间阻塞界面。
  if (!text || text.length > 65536) return
  const state = view.state
  let ranges = state.selection.ranges.filter((range) => !range.empty)
    .map(({ from, to }) => ({ from, to }))
  if (!ranges.length) {
    const lines = [...new Set(state.selection.ranges.map(({ from }) => state.doc.lineAt(from).number))]
    ranges = lines.map((number) => {
      const line = state.doc.line(number)
      return { from: line.from, to: line.to }
    })
  }
  // 若其他扩展改写了复制文本，不输出与纯文本不一致的富文本。
  if (ranges.map(({ from, to }) => state.sliceDoc(from, to)).join(state.lineBreak) !== text) return

  const document = view.dom.ownerDocument
  const pre = document.createElement('pre')
  const base = getComputedStyle(view.contentDOM)
  const editor = getComputedStyle(view.dom)
  pre.style.cssText = 'margin:0;white-space:pre;'
  pre.style.fontFamily = base.fontFamily
  pre.style.fontSize = base.fontSize
  pre.style.lineHeight = base.lineHeight
  pre.style.tabSize = String(state.tabSize)
  pre.style.color = base.color
  pre.style.backgroundColor = editor.getPropertyValue('--card').trim()

  const probe = document.createElement('span')
  probe.style.cssText = 'position:absolute;visibility:hidden;pointer-events:none;'
  view.dom.appendChild(probe)
  const styles = new Map<string, string>()
  const append = (from: number, to: number, classes = '') => {
    if (from >= to) return
    const span = document.createElement('span')
    span.textContent = state.sliceDoc(from, to)
    if (classes) {
      let css = styles.get(classes)
      if (css === undefined) {
        probe.className = classes
        const computed = getComputedStyle(probe)
        css = `color:${computed.color};font-weight:${computed.fontWeight};font-style:${computed.fontStyle};text-decoration:${computed.textDecorationLine};`
        styles.set(classes, css)
      }
      span.style.cssText = css
    }
    pre.appendChild(span)
  }
  try {
    const tree = ensureSyntaxTree(state, ranges[ranges.length - 1].to, 100) ?? syntaxTree(state)
    ranges.forEach(({ from, to }, index) => {
      if (index) pre.appendChild(document.createTextNode(state.lineBreak))
      let position = from
      highlightTree(tree, { style: (tags) => highlightingFor(state, tags) }, (start, end, classes) => {
        append(position, start)
        append(start, end, classes)
        position = end
      }, from, to)
      append(position, to)
    })
    data.setData('text/html', pre.outerHTML)
  } finally {
    probe.remove()
  }
}
