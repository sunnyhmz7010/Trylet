export type PendingAction = { kind: 'new' } | { kind: 'quit'; rememberClose?: boolean } | { kind: 'load'; id: string }

type TransitionOperations<T> = {
  save: () => Promise<unknown>
  stop: () => Promise<unknown>
  quit: (rememberClose: boolean) => Promise<unknown>
  create: () => Promise<T>
  load: (id: string) => Promise<T>
  activate: (draft: T) => void
  refresh: () => Promise<unknown>
}

// 保存失败不得停止运行或丢弃草稿；停止失败不得切换或退出。
export async function performDraftTransition<T>(action: PendingAction, operations: TransitionOperations<T>) {
  await operations.save()
  await operations.stop()
  if (action.kind === 'quit') {
    await operations.quit(!!action.rememberClose)
    return
  }
  const draft = action.kind === 'new' ? await operations.create() : await operations.load(action.id)
  operations.activate(draft)
  await operations.refresh()
}
