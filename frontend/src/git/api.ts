import * as Git from '../../bindings/trylet/internal/trylet/gitreader'
import type { GitError } from './types'

export function errorOf(error: unknown): GitError {
  const raw = error instanceof Error ? error.message : typeof error === 'string' ? error : String((error as {message?: unknown})?.message ?? error)
  const start = raw.indexOf('{"kind"')
  if (start >= 0) {
    try { return JSON.parse(raw.slice(start)) as GitError }
    catch { return { kind: 'git', message: raw } }
  }
  if (error && typeof error === 'object' && 'kind' in error && 'message' in error) return error as GitError
  return { kind: 'git', message: raw }
}

export async function request<T>(command: string, args: Record<string, unknown> = {}): Promise<T> {
  const id = String(args.repoId ?? '')
  let result: unknown
  try {
    switch (command) {
      case 'check_git': result = await Git.CheckGit(); break
      case 'open_repository': result = await Git.OpenRepository(String(args.path ?? '')); break
      case 'activate_repository': result = await Git.ActivateRepository(id, args.remember !== false); break
      case 'recent_repositories': result = await Git.RecentRepositories(); break
      case 'forget_recent_repository': result = await Git.ForgetRecentRepository(String(args.path ?? '')); break
      case 'get_snapshot': result = await Git.GetSnapshot(id); break
      case 'get_history': result = await Git.GetHistory(id, String(args.reference ?? 'all'), Number(args.offset ?? 0), String(args.expected ?? '')); break
      case 'get_commit_view': result = await Git.GetCommitView(id, String(args.commit ?? ''), String(args.path ?? ''), String(args.expectedHistoryRevision ?? '')); break
      case 'get_diff': result = await Git.GetDiff(id, String(args.mode ?? ''), String(args.path ?? ''), String(args.commit ?? ''), String(args.expectedHistoryRevision ?? ''), String(args.expectedChangesRevision ?? '')); break
      case 'get_line_origin': result = await Git.GetLineOrigin(id, args.request as Parameters<typeof Git.GetLineOrigin>[1]); break
      case 'set_git_active': result = await Git.SetActive(Boolean(args.active)); break
      case 'take_launch_request': result = null; break
      default: throw { kind: 'invalid', message: '未实现的只读请求：' + command }
    }
    return result as T
  } catch (error) { throw errorOf(error) }
}

export class RequestGate {
  private serial = 0
  next() { return ++this.serial }
  invalidate() { this.serial++ }
  accepts(ticket: number) { return ticket === this.serial }
}
