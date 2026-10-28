import { useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { Clipboard } from '@wailsio/runtime'
import { errorText } from './lib/language'
import '@xterm/xterm/css/xterm.css'

export type TerminalHandle = {
  write: (data: string | Uint8Array) => void
  clear: () => void
  focus: () => void
  size: () => { cols: number; rows: number }
}
type Props = {
  onReady: (terminal: TerminalHandle) => void
  onInput: (data: string) => void
  onResize: (cols: number, rows: number) => void
}

export default function TerminalView({ onReady, onInput, onResize }: Props) {
  const mount = useRef<HTMLDivElement>(null)
  const callbacks = useRef({ onInput, onResize })
  callbacks.current = { onInput, onResize }
  useEffect(() => {
    const styles = getComputedStyle(document.documentElement)
    const terminal = new Terminal({
      fontFamily: '"Cascadia Code", Consolas, monospace',
      fontSize: 12,
      lineHeight: 1.3,
      cursorBlink: true,
      scrollback: 5000,
      allowProposedApi: false,
      theme: {
        background: styles.getPropertyValue('--card').trim(),
        foreground: styles.getPropertyValue('--foreground').trim(),
        cursor: styles.getPropertyValue('--primary').trim(),
        selectionBackground: '#6b64bb55',
      },
    })
    const fit = new FitAddon()
    terminal.loadAddon(fit)
    terminal.open(mount.current!)
    const resize = () => {
      if (mount.current!.clientWidth > 0 && mount.current!.clientHeight > 0) {
        fit.fit()
        callbacks.current.onResize(terminal.cols, terminal.rows)
      }
    }
    const observer = new ResizeObserver(resize)
    observer.observe(mount.current!)
    resize()
    let disposed = false
    let inputVersion = 0
    let pasting = false
    const pasteClipboard = () => {
      const version = inputVersion
      void Clipboard.Text().then((text) => {
        if (disposed || version !== inputVersion) return
        terminal.focus()
        pasting = true
        try { terminal.paste(text) } finally { pasting = false }
      }).catch((failure) => {
        if (!disposed && version === inputVersion)
          terminal.writeln(`\r\n[粘贴失败：${errorText(failure)}]`)
      })
    }
    const contextMenu = (event: MouseEvent) => {
      event.preventDefault()
      event.stopPropagation()
      pasteClipboard()
    }
    // 在捕获阶段拦截，避免 xterm 的右键处理移动输入区或重复粘贴。
    const container = mount.current!
    container.addEventListener('contextmenu', contextMenu, true)
    const input = terminal.onData((data) => {
      if (data === '\x16' && !pasting) {
        pasteClipboard()
      } else {
        callbacks.current.onInput(data)
      }
    })
    const theme = () => {
      const s = getComputedStyle(document.documentElement)
      terminal.options.theme = {
        background: s.getPropertyValue('--card').trim(),
        foreground: s.getPropertyValue('--foreground').trim(),
        cursor: s.getPropertyValue('--primary').trim(),
      }
    }
    window.addEventListener('trylet:theme-changed', theme)
    onReady({
      write: (data) => terminal.write(data),
      clear: () => { inputVersion++; terminal.reset() },
      focus: () => terminal.focus(),
      size: () => ({ cols: terminal.cols, rows: terminal.rows }),
    })
    return () => {
      disposed = true
      container.removeEventListener('contextmenu', contextMenu, true)
      input.dispose()
      observer.disconnect()
      window.removeEventListener('trylet:theme-changed', theme)
      terminal.dispose()
    }
  }, [])
  return <div ref={mount} className="terminal-mount" />
}
