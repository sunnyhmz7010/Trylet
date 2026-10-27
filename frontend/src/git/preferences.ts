import * as Git from '../../bindings/trylet/internal/trylet/gitreader'

let cache: Record<string,string> = {}
let queue = Promise.resolve()
export async function loadPreferences() {
  cache = {}
  for (const [key,value] of Object.entries((await Git.Preferences()) ?? {})) {
    if (value !== undefined) cache[key] = value
  }
}
export const preferences = {
  getItem(key: string) { return cache[key] ?? null },
  setItem(key: string, value: string) {
    cache[key] = value
    queue = queue.catch((error) => console.error(error)).then(() => Git.SetPreference(key,value))
    void queue.catch((error) => window.dispatchEvent(new CustomEvent('trylet:git-preference-error', {detail: String(error)})))
  },
}
