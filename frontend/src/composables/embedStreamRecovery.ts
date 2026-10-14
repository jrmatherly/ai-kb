type Row = Record<string, unknown>

/** A turn is still open when the reply flag is set or the newest assistant row never got a terminal event. */
export function isTurnOpen(isReplying: boolean, messages: readonly Row[]): boolean {
  const last = messages[messages.length - 1]
  return isReplying || (last?.role === 'assistant' && !last.is_completed)
}

/** True once the newest server row is a finished assistant answer. */
export function serverTurnDone(rows: readonly Row[] | undefined): boolean {
  const last = rows?.[rows.length - 1]
  return last?.role === 'assistant' && last.is_completed === true
}
