<script lang="ts">
  export let open = false
  export let title = ''
  export let message = ''
  export let confirmLabel = 'Delete'
  export let confirmingLabel = 'Working...'
  export let cancelLabel = 'Cancel'
  export let confirmVariant: 'primary' | 'secondary' | 'danger' = 'danger'
  export let confirming = false
  export let onConfirm: () => void = () => {}
  export let onCancel: () => void = () => {}

  let dialog: HTMLDialogElement

  $: if (dialog) {
    if (open && !dialog.open) dialog.showModal()
    else if (!open && dialog.open) dialog.close()
  }

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
