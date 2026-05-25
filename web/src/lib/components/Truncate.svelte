<script lang="ts">
  export let lines = 1
  export let text: string | null | undefined = undefined
  export let tooltip = true

  $: hasText = text !== undefined && text !== null
  $: displayText = hasText ? String(text) : ''
</script>

<span
  class="truncate"
  style:--truncate-lines={lines}
  title={tooltip && hasText ? displayText : undefined}
>
  {#if hasText}{displayText}{:else}<slot />{/if}
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
