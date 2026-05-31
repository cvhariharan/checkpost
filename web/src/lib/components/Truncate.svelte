<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    lines = 1,
    text = undefined,
    tooltip = true,
    children
  }: {
    lines?: number
    text?: string | null
    tooltip?: boolean
    children?: Snippet
  } = $props()

  const hasText = $derived(text !== undefined && text !== null)
  const displayText = $derived(hasText ? String(text) : '')
</script>

<span
  class="truncate"
  style:--truncate-lines={lines}
  title={tooltip && hasText ? displayText : undefined}
>
  {#if hasText}{displayText}{:else if children}{@render children()}{/if}
</span>

<style>
  .truncate {
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: var(--truncate-lines);
    line-clamp: var(--truncate-lines);
    overflow: hidden;
    overflow-wrap: anywhere;
    word-break: break-word;
  }
</style>
