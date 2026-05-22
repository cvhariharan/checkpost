<script>
  export let message = ''
  export let onClose = () => {}
</script>

{#if message}
  <div role="alert" data-variant="error" class="hstack justify-between">
    <p>{message}</p>
    <button type="button" class="small outline" onclick={onClose} aria-label="Dismiss error">Close</button>
  </div>
{/if}
