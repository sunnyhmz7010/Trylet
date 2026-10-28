export type Appearance = 'system' | 'light' | 'dark'

// 主窗口和悬浮球共用外观解析，不加载编辑器或 Git 资源。
export function watchAppearance(preference: string) {
  const media = matchMedia('(prefers-color-scheme: dark)')
  const apply = () => {
    const theme = preference === 'dark' || (preference !== 'light' && media.matches) ? 'dark' : 'light'
    document.documentElement.dataset.theme = theme
    document.documentElement.style.colorScheme = theme
    window.dispatchEvent(new Event('trylet:theme-changed'))
  }
  apply()
  media.addEventListener('change', apply)
  return () => media.removeEventListener('change', apply)
}
