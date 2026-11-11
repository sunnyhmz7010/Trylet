import { useEffect, useState } from 'react'
import { errorText } from './language'

// loader 由模块级常量提供；首次启用后保留组件，切换页面不会丢失表单。
export function useDeferredModule<T>(enabled: boolean, loader: () => Promise<{ default: T }>) {
  const [component, setComponent] = useState<T | null>(null)
  const [error, setError] = useState('')
  const [attempt, setAttempt] = useState(0)
  useEffect(() => {
    if (!enabled || component || error) return
    let disposed = false
    void loader().then((module) => {
      if (!disposed) setComponent(() => module.default)
    }).catch((failure) => {
      if (!disposed) setError(errorText(failure))
    })
    return () => { disposed = true }
  }, [enabled, loader, component, error, attempt])
  const retry = () => { setError(''); setAttempt((value) => value + 1) }
  return { component, error, retry }
}
