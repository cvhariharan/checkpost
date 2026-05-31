<script lang="ts">
  import Pagination from './Pagination.svelte'
  import { formatTimestamp } from '$lib/util'
  import type { Schedule, ScheduleResultRow } from '$lib/api'

  let {
    schedule = null,
    columns = [],
    rows = [],
    total = 0,
    page = 1,
    pageCount = 1,
    countPerPage = 500,
    loading = false,
    query = '',
    lastRefreshed = '',
    onClose = () => {},
    onRefresh = () => {},
    onApplyQuery = () => {},
    onPageChange = () => {}
  }: {
    schedule?: Schedule | null
    columns?: string[]
    rows?: ScheduleResultRow[]
    total?: number
    page?: number
    pageCount?: number
    countPerPage?: number
    loading?: boolean
    query?: string
    lastRefreshed?: string
    onClose?: () => void
    onRefresh?: () => void
    onApplyQuery?: (query: string) => void
    onPageChange?: (page: number) => void
  } = $props()

  const rowHeight = 36
  const headerHeight = 40
  const overscan = 12

  let draftQuery = $state('')
  let lastSyncedQuery = $state('')
  let scrollTop = $state(0)
  let viewportHeight = $state(520)

  $effect(() => {
    if (query === lastSyncedQuery) return
    draftQuery = query
    lastSyncedQuery = query
  })
  const templateColumns = $derived(`220px 180px repeat(${columns.length}, 220px)`)
  const startIndex = $derived(
    Math.max(0, Math.floor(Math.max(0, scrollTop - headerHeight) / rowHeight) - overscan)
  )
  const visibleCount = $derived(Math.ceil(viewportHeight / rowHeight) + overscan * 2)
  const endIndex = $derived(Math.min(rows.length, startIndex + visibleCount))
  const visibleRows = $derived(rows.slice(startIndex, endIndex))
  const bodyHeight = $derived(headerHeight + rows.length * rowHeight)
  const rangeStart = $derived(total === 0 ? 0 : (page - 1) * countPerPage + 1)
  const rangeEnd = $derived(Math.min(page * countPerPage, total))
  const queryError = $derived(validateQuery(draftQuery))

  function handleScroll(event: UIEvent) {
    scrollTop = (event.currentTarget as HTMLElement).scrollTop
  }

  function applyQuery() {
    if (queryError) return
    onApplyQuery(draftQuery.trim())
  }

  function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    applyQuery()
  }

  function validateQuery(value: string) {
    let inQuote = false
    let escaped = false
    for (const char of value) {
      if (escaped) escaped = false
      else if (char === '\\' && inQuote) escaped = true
      else if (char === '"') inQuote = !inQuote
    }
    return inQuote ? 'Close the quoted value before searching.' : ''
  }
</script>

<section class="vstack gap-3">
  <header class="hstack justify-between">
    <div>
      <h2>{schedule?.title || 'Schedule results'}</h2>
      <p class="text-light">{schedule?.sql || ''}</p>
    </div>
    <menu class="buttons">
      <li><button type="button" class="small outline" disabled={loading} onclick={onRefresh}>Refresh</button></li>
      <li><button type="button" class="small outline" onclick={onClose}>Close</button></li>
    </menu>
  </header>

  <form onsubmit={handleSubmit}>
    <fieldset class="group">
      <input
        type="search"
        bind:value={draftQuery}
        aria-invalid={queryError ? 'true' : undefined}
        placeholder="Search results (e.g. name:sudo machine:laptop)"
        disabled={loading}
      />
      <button type="submit" disabled={loading || !!queryError}>Search</button>
    </fieldset>
  </form>
  {#if queryError}
    <p role="alert" data-variant="error">{queryError}</p>
  {/if}

  <div class="hstack justify-between">
    <p class="text-light">
      Showing <strong>{rangeStart}</strong> to <strong>{rangeEnd}</strong> of <strong>{total}</strong> rows
    </p>
    {#if lastRefreshed}
      <p class="text-light">Refreshed {lastRefreshed}</p>
    {/if}
  </div>

  <div
    class="table virtual-table"
    role="region"
    aria-label={`${schedule?.title || 'Schedule'} results`}
    aria-busy={loading ? 'true' : undefined}
    bind:clientHeight={viewportHeight}
    onscroll={handleScroll}
  >
    <div class="result-grid" style={`grid-template-columns: ${templateColumns}; height: ${bodyHeight}px;`}>
      <div class="result-header" style={`grid-template-columns: ${templateColumns};`}>
        <div class="cell sticky-left first">Machine</div>
        <div class="cell sticky-left second">Last seen</div>
        {#each columns as column}
          <div class="cell" title={column}>{column}</div>
        {/each}
      </div>

      {#if loading}
        <div class="state-row" style={`top: ${headerHeight}px;`}>Loading results...</div>
      {:else if rows.length === 0}
        <div class="state-row" style={`top: ${headerHeight}px;`}>No results yet</div>
      {:else}
        {#each visibleRows as row, offset}
          <div
            class="result-row"
            style={`grid-template-columns: ${templateColumns}; transform: translateY(${headerHeight + (startIndex + offset) * rowHeight}px);`}
          >
            <div class="cell sticky-left first" title={row.hostname || row.node_uuid}>{row.hostname || row.node_uuid}</div>
            <div class="cell sticky-left second" title={formatTimestamp(row.last_seen)}>{formatTimestamp(row.last_seen)}</div>
            {#each columns as column}
              <div class="cell" title={String(row.columns?.[column] ?? '')}>{row.columns?.[column] ?? ''}</div>
            {/each}
          </div>
        {/each}
      {/if}
    </div>
  </div>

  <footer class="hstack justify-between">
    <span class="text-light">{rows.length} loaded on this page</span>
    <Pagination currentPage={page} {pageCount} disabled={loading} label="Schedule results pagination" onPageChange={onPageChange} />
  </footer>
</section>

<style>
  .virtual-table {
    height: min(62vh, 640px);
    min-height: 360px;
    overflow: auto;
    position: relative;
  }
  .result-grid {
    display: grid;
    min-width: max-content;
    position: relative;
  }
  .result-header,
  .result-row {
    display: grid;
    left: 0;
    min-width: max-content;
    position: absolute;
    right: 0;
  }
  .result-header {
    background: var(--background, Canvas);
    height: 40px;
    position: sticky;
    top: 0;
    z-index: 4;
  }
  .result-row {
    height: 36px;
  }
  .cell {
    align-items: center;
    border-bottom: 1px solid var(--border);
    display: flex;
    min-width: 0;
    overflow: hidden;
    padding: 0 0.6rem;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .result-header .cell {
    font-weight: 600;
  }
  .sticky-left {
    background: var(--background, Canvas);
    position: sticky;
    z-index: 2;
  }
  .first {
    left: 0;
  }
  .second {
    left: 220px;
  }
  .result-header .sticky-left {
    z-index: 5;
  }
  .state-row {
    left: 0;
    padding: 1rem;
    position: absolute;
    right: 0;
    text-align: center;
  }
</style>
