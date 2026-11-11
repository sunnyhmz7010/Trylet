import { Maximize2, Minimize2, Minus, X } from 'lucide-react'
import type { WindowActions } from './useWindowActions'
export default function WindowControls({ actions }: { actions: WindowActions }) {
  const { largeWindow, minimise, toggleWindowMode, closeWindow } = actions
  return <div className="window-controls" aria-label="窗口操作">
    <button className="icon-button" aria-label="最小化" title="最小化到任务栏" onClick={minimise}><Minus size={17} /></button>
    <button className="icon-button" aria-label={largeWindow ? '还原小窗口' : '切换到大窗口'} title={largeWindow ? '还原小窗口' : '大窗口（最大化）'} onClick={toggleWindowMode}>{largeWindow ? <Minimize2 size={15} /> : <Maximize2 size={15} />}</button>
    <button className="icon-button window-close" aria-label="关闭窗口" title="关闭窗口" onClick={closeWindow}><X size={17} /></button>
  </div>
}
