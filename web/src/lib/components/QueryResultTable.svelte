<script lang="ts">
  // Renders a single ad-hoc query result as a table (with per-row JSON expand),
  // falling back to raw text / empty / pending states. Shared by the machine
  // detail Query tab and the multi-host query run view so rendering stays identical.
  type ResultView =
    | { type: 'pending' }
    | { type: 'empty'; message: string }
    | { type: 'table'; rows: Record<string, unknown>[]; columns: string[] }
    | { type: 'fallback'; text: string }

  let {
    results,
    error = '',
    keyPrefix = '',
    pendingLabel = 'Awaiting results...'
  }: {
    results: unknown
    error?: string
    keyPrefix?: string
    pendingLabel?: string
  } = $props()

  let expandedResultRowKey = $state('')

  const view = $derived<ResultView>(
    results === undefined || results === null ? { type: 'pending' } : resultView(results)
  )

  function formatResults(value: unknown): string {
    if (value === undefined || value === null) return pendingLabel
    try {
      return typeof value === 'string' ? value : JSON.stringify(value, null, 2)
    } catch {
      return String(value)
    }
  }

  function resultView(value: unknown): ResultView {
    const parsed = parseStringResult(value)
    if (parsed !== value) return resultView(parsed)

    if (Array.isArray(value)) {
      if (value.length === 0) return { type: 'empty', message: 'No rows returned' }
      if (value.every(isRowObject)) return tableView(value as Record<string, unknown>[])
      return { type: 'fallback', text: formatResults(value) }
    }

    if (isPlainObject(value)) {
      const obj = value as Record<string, unknown>
      for (const key of ['rows', 'results', 'data']) {
        if (Array.isArray(obj[key])) return resultView(obj[key])
      }
      if (isRowObject(value)) return tableView([obj])
      return { type: 'fallback', text: formatResults(value) }
    }

    return { type: 'fallback', text: formatResults(value) }
  }

  function tableView(rows: Record<string, unknown>[]): ResultView {
    const columns = resultColumns(rows)
    if (columns.length === 0) return { type: 'empty', message: 'Rows returned without columns' }
    return { type: 'table', rows, columns }
  }

  function resultColumns(rows: Record<string, unknown>[]): string[] {
    const seen = new Set<string>()
    const columns: string[] = []
    rows.forEach((row) => {
      Object.keys(row).forEach((column) => {
        if (!seen.has(column)) {
          seen.add(column)
          columns.push(column)
        }
      })
    })
    return columns
  }

  function formatCellValue(value: unknown): string {
    if (value === undefined) return ''
    if (value === null) return 'null'
    if (typeof value === 'object') return formatResults(value)
    return String(value)
  }

  function resultRowKey(rowIndex: number): string {
    return `${keyPrefix || 'result'}:${rowIndex}`
  }

  function toggleResultRowByKey(key: string) {
    expandedResultRowKey = expandedResultRowKey === key ? '' : key
  }

  function handleResultRowKeydown(event: KeyboardEvent, rowIndex: number) {
    if (event.key !== 'Enter' && event.key !== ' ') return
    event.preventDefault()
    toggleResultRowByKey(resultRowKey(rowIndex))
  }

  function parseStringResult(value: unknown): unknown {
    if (typeof value !== 'string') return value
    const trimmed = value.trim()
    if (!trimmed || !['[', '{'].includes(trimmed[0])) return value
    try {
      return JSON.parse(trimmed)
    } catch {
      return value
    }
  }

  function isPlainObject(value: unknown): value is Record<string, unknown> {
    return value !== null && typeof value === 'object' && !Array.isArray(value)
  }

  function isRowObject(value: unknown): boolean {
    return isPlainObject(value) && Object.values(value).every((cell) => cell === null || typeof cell !== 'object')
  }
</script>

{#if error}
  <pre class="result-fallback"><code>{error}</code></pre>
{:else if view.type === 'pending'}
  <p class="text-light result-message">{pendingLabel}</p>
{:else if view.type === 'empty'}
  <p class="text-light result-message">{view.message}</p>
{:else if view.type === 'table'}
  <div class="table query-results-table">
    <table>
      <thead>
        <tr>
          {#each view.columns as column}
            <th class="result-header">{column}</th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each view.rows as row, rowIndex}
          {@const rowKey = resultRowKey(rowIndex)}
          <tr
            class="result-row"
            class:expanded-result-row={expandedResultRowKey === rowKey}
            tabindex="0"
            aria-expanded={expandedResultRowKey === rowKey}
            title="Click to show full row"
            onclick={() => toggleResultRowByKey(rowKey)}
            onkeydown={(e) => handleResultRowKeydown(e, rowIndex)}
          >
            {#each view.columns as column}
              <td class="result-cell" title={formatCellValue(row[column])}>
                <span class="result-cell-content">{formatCellValue(row[column])}</span>
              </td>
            {/each}
          </tr>
          {#if expandedResultRowKey === rowKey}
            <tr class="result-row-details">
              <td colspan={view.columns.length}>
                <pre class="result-row-json"><code>{formatResults(row)}</code></pre>
              </td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>
  </div>
{:else}
  <pre class="result-fallback"><code>{view.text}</code></pre>
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
</style>
