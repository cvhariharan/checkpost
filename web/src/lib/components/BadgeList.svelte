<script lang="ts">
  type Item = string | { name?: string; label?: string }

  export let items: Item[] = []
  export let max = 2
  export let outline = true
  export let empty = '—'

  $: names = (items || [])
    .map((item) => (typeof item === 'string' ? item : item?.name || item?.label || ''))
    .filter((name) => name.length > 0)
  $: visible = names.slice(0, max)
  $: overflow = Math.max(0, names.length - max)
  $: tooltip = names.join(', ')
</script>

{#if names.length === 0}
  <span class="text-light">{empty}</span>
{:else}
  <span class="hstack gap-1 badge-list" title={tooltip}>
    {#each visible as name}
      <span class="badge" class:outline>{name}</span>
    {/each}
    {#if overflow > 0}
      <span class="badge" class:outline aria-label={`${overflow} more`}>+{overflow}</span>
    {/if}
  </span>
{/if}

<style>
  .badge-list {
    flex-wrap: nowrap;
    min-width: 0;
  }
  .badge-list > .badge {
    max-width: 8rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
