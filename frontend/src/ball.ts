import { Appearance, RefreshClipboard, SyncBallDPI, ToggleEditor } from '../bindings/trylet/desktop'
import brandGlyph from './brand.svg?raw'
import './ball.css'
import { Events } from '@wailsio/runtime'
import { watchAppearance } from './appearance'

const ball = document.querySelector<HTMLButtonElement>('#ball')!
ball.innerHTML = `<span id="ball-counts" hidden><span>计空格</span><b id="with-spaces"></b><span>不计空格</span><b id="without-spaces"></b></span><span class="ball-icon">${brandGlyph}</span>`
const counts = document.querySelector<HTMLElement>('#ball-counts')!
const withSpaces = document.querySelector<HTMLElement>('#with-spaces')!
const withoutSpaces = document.querySelector<HTMLElement>('#without-spaces')!
Events.On('trylet:clipboard-count', ({ data }) => {
  ball.classList.toggle('counting', data.hasText)
  counts.hidden = !data.hasText
  withSpaces.textContent = data.withSpaces.toLocaleString('zh-CN')
  withoutSpaces.textContent = data.withoutSpaces.toLocaleString('zh-CN')
  const summary = data.hasText ? `上次复制的字符数：计空格 ${withSpaces.textContent}，不计空格 ${withoutSpaces.textContent}。` : ''
  ball.title = data.error || `${summary}Trylet · 点击打开，拖动移动`
  ball.setAttribute('aria-label', `${summary}打开 Trylet`)
})
void RefreshClipboard().catch((error: unknown) => { ball.title = `统计剪贴板失败：${String(error)}` })
let stopAppearance = watchAppearance('system')
let appearanceVersion = 0
const applyAppearance = (value: string) => { stopAppearance(); stopAppearance = watchAppearance(value) }
Events.On('trylet:appearance', (event) => { appearanceVersion++; applyAppearance(String(event.data ?? 'system')) })
const initialAppearanceVersion = appearanceVersion
void Appearance().then((value) => {
  if (appearanceVersion === initialAppearanceVersion) applyAppearance(value)
}).catch((error: unknown) => { ball.title = `读取外观设置失败：${String(error)}` })
const watchDPI = () => {
  void SyncBallDPI().catch((error: unknown) => {
    ball.title = `调整显示缩放失败：${String(error)}`
  })
  matchMedia(`(resolution: ${window.devicePixelRatio}dppx)`)
    .addEventListener('change', watchDPI, { once: true })
}
watchDPI()
let startX = 0,
  startY = 0,
  moved = false
ball.addEventListener('pointerdown', (event) => {
  startX = event.screenX
  startY = event.screenY
  moved = false
})
ball.addEventListener('pointermove', (event) => {
  if (
    event.buttons === 1 &&
    !moved &&
    Math.hypot(event.screenX - startX, event.screenY - startY) > 4
  ) {
    moved = true
  }
})
ball.addEventListener('click', () => {
  if (!moved)
    void ToggleEditor().catch((error: unknown) => {
      ball.title = `打开失败：${String(error)}`
    })
})
ball.addEventListener('keydown', (event) => {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    void ToggleEditor()
  }
})
