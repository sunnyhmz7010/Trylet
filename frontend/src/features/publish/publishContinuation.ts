// 待续发布属于发起时的草稿；切页取消、草稿变化或成功消费后均不可复用。
export class PublishContinuation {
  private owner: string | null = null

  request(draftID: string) { this.owner = draftID }
  cancel() { this.owner = null }
  consume(draftID: string | undefined, configured: boolean): boolean {
    if (!this.owner) return false
    if (this.owner !== draftID) { this.cancel(); return false }
    if (!configured) return false
    this.cancel()
    return true
  }
}
