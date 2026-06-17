<script lang="ts">
  // Presentational table for one server-paginated page of query result rows.
  import Pagination from './Pagination.svelte'

  let {
    columns = [],
    rows = [],
    total = 0,
    page = 1,
    pageCount = 1,
    loading = false,
    pending = false,
    error = '',
    browsingDisabled = false,
    pendingLabel = 'Awaiting results...',
    onPageChange = () => {}
  }: {
    columns?: string[]
    rows?: Record<string, string>[]
    total?: number
    page?: number
    pageCount?: number
    loading?: boolean
    pending?: boolean
    error?: string
    browsingDisabled?: boolean
    pendingLabel?: string
    onPageChange?: (page: number) => void
  } = $props()

  let expandedRow = $state(-1)

  function toggleRow(index: number) {
    expandedRow = expandedRow === index ? -1 : index
  }

  function handleRowKeydown(event: KeyboardEvent, index: number) {
    if (event.key !== 'Enter' && event.key !== ' ') return
    event.preventDefault()
    toggleRow(index)
  }

  function cellValue(value: unknown): string {
    if (value === undefined || value === null) return ''
    return String(value)
  }

  function rowJSON(row: Record<string, string>): string {
    try {
      return JSON.stringify(row, null, 2)
    } catch {
      return String(row)
    }
  }
</script>

{#if error}
  <pre class="result-fallback"><code>{error}</code></pre>
{:else if browsingDisabled}
  <p class="text-light result-message">Result browsing is disabled on this server.</p>
{:else if pending}
  <p class="text-light result-message">{pendingLabel}</p>
{:else if loading && rows.length === 0}
  <p class="text-light result-message">Loading results...</p>
{:else if rows.length === 0}
  <p class="text-light result-message">No rows returned</p>
{:else}
  <div class="table query-results-table" aria-busy={loading ? 'true' : undefined}>
    <table>
      <thead>
        <tr>
          {#each columns as column}
            <th class="result-header">{column}</th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each rows as row, rowIndex}
          <tr
            class="result-row"
            class:expanded-result-row={expandedRow === rowIndex}
            tabindex="0"
            aria-expanded={expandedRow === rowIndex}
            title="Click to show full row"
            onclick={() => toggleRow(rowIndex)}
            onkeydown={(e) => handleRowKeydown(e, rowIndex)}
          >
            {#each columns as column}
              <td class="result-cell" title={cellValue(row[column])}>
                <span class="result-cell-content">{cellValue(row[column])}</span>
              </td>
            {/each}
          </tr>
          {#if expandedRow === rowIndex}
            <tr class="result-row-details">
              <td colspan={columns.length}>
                <pre class="result-row-json"><code>{rowJSON(row)}</code></pre>
              </td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>
  </div>
  <footer class="hstack justify-between result-footer">
    <span class="text-light">{total} row{total === 1 ? '' : 's'}</span>
    <Pagination currentPage={page} {pageCount} disabled={loading} label="Query results pagination" {onPageChange} />
  </footer>
{/if}

<style>
  .query-results-table {
    margin-top: var(--space-3, 1rem);
  }
  .query-results-table table {
    width: max-content;
    min-width: 100%;
    table-layout: fixed;
  }
  .result-header,
  .result-cell {
    width: 14rem;
    min-width: 14rem;
    max-width: 14rem;
  }
  .expanded-result-row {
    background-color: rgb(from var(--muted) r g b / 0.35);
  }
  .result-row {
    cursor: pointer;
  }
  .result-row:focus-visible {
    outline: 2px solid var(--primary);
    outline-offset: -2px;
  }
  .result-cell {
    vertical-align: top;
  }
  .result-cell-content {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .result-row-json {
    margin: 0;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .result-message {
    margin-top: var(--space-3, 1rem);
  }
  .result-fallback {
    margin-top: var(--space-3, 1rem);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .result-footer {
    margin-top: var(--space-3, 1rem);
  }
</style>
