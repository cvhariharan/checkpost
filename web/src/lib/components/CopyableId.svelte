<script lang="ts">
  import { toast } from '$lib/util'
  import Copy from '@lucide/svelte/icons/copy'

  let {
    value = '',
    label = 'UUID'
  }: {
    value?: string
    label?: string
  } = $props()

  async function copy() {
    if (!value) return
    try {
      await navigator.clipboard.writeText(value)
      toast(`${label} copied`, 'Clipboard', { variant: 'success' })
    } catch {
      toast(`Could not copy ${label.toLowerCase()}`, 'Clipboard', { variant: 'danger' })
    }
  }
</script>

{#if value}
  <span class="copyable-id">
    <code>{value}</code>
    <button
      type="button"
      class="small outline copyable-id-btn"
      onclick={copy}
      title={`Copy ${label.toLowerCase()}`}
      aria-label={`Copy ${label.toLowerCase()}`}
    >
      <Copy size={14} aria-hidden="true" />
    </button>
  </span>
{/if}

<style>
  .copyable-id {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.85rem;
  }
  .copyable-id code {
    font-size: 0.85em;
  }
  .copyable-id-btn {
    display: inline-flex;
    align-items: center;
    padding: 0.1rem 0.3rem;
  }
</style>
