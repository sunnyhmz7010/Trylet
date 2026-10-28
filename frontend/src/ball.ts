import { Appearance, SyncBallDPI, ToggleEditor } from '../bindings/trylet/desktop'
import brandGlyph from './brand.svg?raw'
import './ball.css'
import { Events } from '@wailsio/runtime'
import { watchAppearance } from './appearance'

const ball = document.querySelector<HTMLButtonElement>('#ball')!
ball.innerHTML = brandGlyph
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
