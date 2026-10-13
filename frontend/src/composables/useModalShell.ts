import { onBeforeUnmount, onMounted } from 'vue'
import { DialogPlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'

/**
 * Shell behavior shared by the full-screen "settings-style" dialogs (Settings / AgentEditor / OrganizationSettings / KnowledgeBaseEditor):
 *   - Esc closes (ignored while a TDesign dialog / drawer / popup is open on top, so those close first)
 *   - Clicking the overlay closes
 *   - Unsaved changes ask for confirmation first
 *
 * Usage:
 *   const shell = useModalShell({
 *     visible: () => props.visible,
 *     close: () => emit('update:visible', false),
 *     snapshot: () => formData.value,   // optional: data compared for dirty state
 *   })
 *   // After data loads or a save succeeds: shell.markClean()
 *   // Template: @click.self="shell.requestClose", close button @click="shell.requestClose"
 */
export interface ModalShellOptions {
  visible: () => boolean
  close: () => void
  /** Data compared for dirty state; without it there is no unsaved-changes prompt */
  snapshot?: () => unknown
  /** The form is not editable while loading, and initial assignment does not count as an unsaved change */
  loading?: () => boolean
  /** Skip Esc when it returns true (for example while a popup drawn by the component itself is open) */
  ignoreEscape?: () => boolean
}

function serialize(value: unknown): string {
  try {
    return JSON.stringify(value ?? null)
  } catch {
    return ''
  }
}

/** Whether a TDesign dialog / drawer / popup is open on the page */
function hasOpenTDesignOverlay(): boolean {
  if (typeof document === 'undefined') return false
  const dialogCtx = document.querySelector<HTMLElement>('.t-dialog__ctx')
  if (dialogCtx && dialogCtx.style.display !== 'none') return true
  if (document.querySelector('.t-drawer--open')) return true
  const popups = document.querySelectorAll<HTMLElement>('.t-popup')
  for (const popup of popups) {
    if (popup.style.display !== 'none') return true
  }
  return false
}

export function useModalShell(options: ModalShellOptions) {
  const { t } = useI18n()
  let cleanSnapshot = serialize(options.snapshot?.())
  let confirming = false

  const markClean = () => {
    cleanSnapshot = serialize(options.snapshot?.())
  }

  const isDirty = () => {
    if (!options.snapshot || options.loading?.()) return false
    return serialize(options.snapshot()) !== cleanSnapshot
  }

  // Template handlers pass the DOM event as the first argument, so only call
  // afterClose when it really is a callback.
  const requestClose = (afterClose?: unknown) => {
    if (!options.visible() || confirming) return
    const close = () => {
      options.close()
      if (typeof afterClose === 'function') afterClose()
    }
    if (!isDirty()) {
      close()
      return
    }
    confirming = true
    const dialog = DialogPlugin.confirm({
      header: t('common.unsavedChanges.title'),
      body: t('common.unsavedChanges.body'),
      theme: 'warning',
      confirmBtn: { content: t('common.unsavedChanges.discard'), theme: 'danger' },
      cancelBtn: t('common.unsavedChanges.keepEditing'),
      onConfirm: () => {
        confirming = false
        dialog.destroy()
        close()
      },
      onClose: () => {
        confirming = false
        dialog.destroy()
      },
    })
  }

  const onKeydown = (event: KeyboardEvent) => {
    if (event.key !== 'Escape' || !options.visible()) return
    if (options.ignoreEscape?.()) return
    if (hasOpenTDesignOverlay()) return
    requestClose()
  }

  onMounted(() => window.addEventListener('keydown', onKeydown))
  onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

  return { requestClose, markClean, isDirty }
}
