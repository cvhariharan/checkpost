<script lang="ts">
  let {
    open = $bindable(false),
    title = '',
    message = '',
    confirmLabel = 'Delete',
    confirmingLabel = 'Working...',
    cancelLabel = 'Cancel',
    confirmVariant = 'danger',
    confirming = false,
    onConfirm = () => {},
    onCancel = () => {}
  }: {
    open?: boolean
    title?: string
    message?: string
    confirmLabel?: string
    confirmingLabel?: string
    cancelLabel?: string
    confirmVariant?: 'primary' | 'secondary' | 'danger'
    confirming?: boolean
    onConfirm?: () => void
    onCancel?: () => void
  } = $props()

  let dialog = $state<HTMLDialogElement>()

  $effect(() => {
    if (!dialog) return
    if (open && !dialog.open) dialog.showModal()
    else if (!open && dialog.open) dialog.close()
  })

  function handleCancel() {
    open = false
    onCancel()
  }

  function handleClose() {
    if (open) {
      open = false
      onCancel()
    }
  }
</script>

<dialog bind:this={dialog} closedby="any" onclose={handleClose}>
  <form method="dialog">
    <header>
      <h2>{title}</h2>
      {#if message}<p>{message}</p>{/if}
    </header>
    <footer>
      <button type="button" class="outline" onclick={handleCancel}>{cancelLabel}</button>
      <button type="button" data-variant={confirmVariant} disabled={confirming} onclick={onConfirm}>
        {confirming ? confirmingLabel : confirmLabel}
      </button>
    </footer>
  </form>
</dialog>
