<script lang="ts" generics="C extends string">
  import { nextSort, type SortState } from '$lib/tableSort'

  let {
    state = $bindable(),
    col,
    label,
    align = 'left'
  }: {
    state: SortState<C>
    col: C
    label: string
    align?: 'left' | 'right' | 'center'
  } = $props()

  const active = $derived(state?.col === col)
  const indicator = $derived(active ? (state?.dir === 'asc' ? ' ▲' : ' ▼') : '')
  const ariaSort = $derived(
    !active ? 'none' : state?.dir === 'asc' ? 'ascending' : 'descending'
  ) as 'ascending' | 'descending' | 'none'
</script>

<th
  class:align-right={align === 'right'}
  class:align-center={align === 'center'}
  aria-sort={ariaSort}
>
  <button
    type="button"
    class="sort-header"
    aria-label={`Sort by ${label}`}
    onclick={() => (state = nextSort(state, col))}
  >
    {label}{indicator}
  </button>
</th>

<style>
  .sort-header {
    background: none;
    border: none;
    padding: 0;
    margin: 0;
    font: inherit;
    color: inherit;
    cursor: pointer;
    white-space: nowrap;
  }
</style>
