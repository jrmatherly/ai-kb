const editableInputTypes = new Set(['text', 'search', 'url', 'email', 'password', 'number', 'tel', ''])

// Text fields, textareas and contenteditable hosts: focusing the guide would
// take the caret away from them.
export const isEditable = (el: Element | null): boolean => {
  if (!el) return false
  if ((el as HTMLElement).isContentEditable) return true
  const tag = el.tagName
  if (tag === 'TEXTAREA') return true
  if (tag === 'INPUT') return editableInputTypes.has(((el as HTMLInputElement).type || '').toLowerCase())
  return false
}
