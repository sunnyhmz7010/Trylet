import { Events } from '@wailsio/runtime'
import { ChooseGitRepository } from '../../bindings/trylet/desktop'

export async function open(_options?: unknown): Promise<string | null> {
  return (await ChooseGitRepository()) || null
}
export async function listen<T>(name: string, callback: (event: {payload: T}) => void): Promise<() => void> {
  if (name === 'repository-invalidated') {
    return Events.On('trylet:git-changed', (event) => callback({payload: event.data as T}))
  }
  return () => undefined
}
