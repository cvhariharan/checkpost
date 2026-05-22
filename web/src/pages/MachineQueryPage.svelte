<script>
  import { onMount } from 'svelte'
  import { deleteMachineQuery, executeMachineQuery, fetchMachine, fetchMachinePolicies, fetchMachineQueries } from '@/api.js'
  import ConfirmDialog from '@/components/common/ConfirmDialog.svelte'
  import ErrorMessage from '@/components/common/ErrorMessage.svelte'
  import OatPagination from '@/components/common/OatPagination.svelte'

  export let params = {}

  let machine = null
  let queryText = ''
  let queryHistory = []
  let policyPosture = []
  let loading = true
  let historyLoading = false
  let executing = false
  let error = ''
  let expandedResultRowKey = ''
  let currentQueryPage = 1
  let queryPageCount = 1
  let queryTotalCount = 0
  const queryCountPerPage = 10
  let deleteDialogOpen = false
  let queryToDelete = null
  let deletingQuery = false

  $: machineId = params.id
  $: queryStartResult = queryTotalCount === 0 ? 0 : (currentQueryPage - 1) * queryCountPerPage + 1
  $: queryEndResult = Math.min(currentQueryPage * queryCountPerPage, queryTotalCount)

  onMount(loadMachine)

  async function loadMachine() {
    loading = true
    error = ''
    try {
      const [machineData, historyData, policyData] = await Promise.all([
        fetchMachine(machineId),
        fetchMachineQueries(machineId, { page: currentQueryPage, countPerPage: queryCountPerPage }),
        fetchMachinePolicies(machineId)
      ])
      machine = machineData
      setQueryHistory(historyData)
      policyPosture = Array.isArray(policyData) ? policyData : policyData.policies || []
    } catch (err) {
      error = err.message || 'Failed to load machine data'
    } finally {
      loading = false
    }
  }

  function setQueryHistory(data) {
    queryHistory = Array.isArray(data) ? data : data.queries || []
    queryTotalCount = Array.isArray(data) ? queryHistory.length : data.total_count || 0
    queryPageCount = Math.max(1, Array.isArray(data) ? 1 : data.page_count || 1)
  }

  async function loadQueryHistory(page = currentQueryPage) {
    historyLoading = true
    error = ''
    try {
      const data = await fetchMachineQueries(machineId, { page, countPerPage: queryCountPerPage })
      currentQueryPage = page
      expandedResultRowKey = ''
      setQueryHistory(data)
    } catch (err) {
      error = err.message || 'Failed to load query history'
    } finally {
      historyLoading = false
    }
  }

  async function runQuery() {
    if (!queryText.trim()) return
    executing = true
    error = ''
    try {
      await executeMachineQuery(machineId, queryText)
      queryText = ''
      await loadQueryHistory(1)
      setTimeout(() => loadQueryHistory(1), 6000)
    } catch (err) {
      error = err.message || 'Query execution failed'
    } finally {
      executing = false
    }
  }

  async function changeQueryPage(page) {
    if (page < 1 || page > queryPageCount || page === currentQueryPage || historyLoading) {
      return
    }
    await loadQueryHistory(page)
  }

  function confirmDeleteQuery(query) {
    if (!query?.id) return
    queryToDelete = query
    deleteDialogOpen = true
  }

  async function deleteSelectedQuery() {
    if (!queryToDelete?.id) return
    deletingQuery = true
    error = ''
    try {
      await deleteMachineQuery(machineId, queryToDelete.id)
      const targetPage = queryHistory.length === 1 && currentQueryPage > 1
        ? currentQueryPage - 1
        : currentQueryPage
      deleteDialogOpen = false
      queryToDelete = null
      await loadQueryHistory(targetPage)
    } catch (err) {
      error = err.message || 'Failed to delete query'
    } finally {
      deletingQuery = false
    }
  }

  function cancelDeleteQuery() {
    queryToDelete = null
  }

  function formatTimestamp(ts) {
    if (!ts) return ''
    try {
      return new Date(ts).toLocaleString()
    } catch {
      return ts
    }
  }

  function formatResults(results) {
    if (results === undefined || results === null) return 'Awaiting results...'
    try {
      return typeof results === 'string' ? results : JSON.stringify(results, null, 2)
    } catch {
      return String(results)
    }
  }

  function queryResultView(query) {
    if (query?.results === undefined || query?.results === null) {
      return { type: 'pending' }
    }

    return resultView(query.results)
  }

  function resultView(results) {
    const parsed = parseStringResult(results)
    if (parsed !== results) {
      return resultView(parsed)
    }

    if (Array.isArray(results)) {
      if (results.length === 0) {
        return { type: 'empty', message: 'No rows returned' }
      }

      if (results.every(isRowObject)) {
        return tableView(results)
      }

      return { type: 'fallback', text: formatResults(results) }
    }

    if (isPlainObject(results)) {
      for (const key of ['rows', 'results', 'data']) {
        if (Array.isArray(results[key])) {
          return resultView(results[key])
        }
      }

      if (isRowObject(results)) {
        return tableView([results])
      }

      return { type: 'fallback', text: formatResults(results) }
    }

    return { type: 'fallback', text: formatResults(results) }
  }

  function tableView(rows) {
    const columns = resultColumns(rows)
    if (columns.length === 0) {
      return { type: 'empty', message: 'Rows returned without columns' }
    }

    return {
      type: 'table',
      rows,
      columns
    }
  }

  function resultColumns(rows) {
    const seen = new Set()
    const columns = []

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

  function formatCellValue(value) {
    if (value === undefined) {
      return ''
    }
    if (value === null) {
      return 'null'
    }
    if (typeof value === 'object') {
      return formatResults(value)
    }

    return String(value)
  }

  function resultRowKey(query, rowIndex) {
    return `${query?.id || query?.timestamp || query?.query || 'query'}:${rowIndex}`
  }

  function toggleResultRowByKey(key) {
    expandedResultRowKey = expandedResultRowKey === key ? '' : key
  }

  function handleResultRowKeydown(event, query, rowIndex) {
    if (event.key !== 'Enter' && event.key !== ' ') {
      return
    }
    event.preventDefault()
    toggleResultRowByKey(resultRowKey(query, rowIndex))
  }

  function parseStringResult(value) {
    if (typeof value !== 'string') {
      return value
    }

    const trimmed = value.trim()
    if (!trimmed || !['[', '{'].includes(trimmed[0])) {
      return value
    }

    try {
      return JSON.parse(trimmed)
    } catch {
      return value
    }
  }

  function isPlainObject(value) {
    return value !== null && typeof value === 'object' && !Array.isArray(value)
  }

  function isRowObject(value) {
    return isPlainObject(value) && Object.values(value).every((cell) => cell === null || typeof cell !== 'object')
  }

  function isOnline(machine) {
    const timestamp = machine?.last_seen_at || machine?.enrolled_at
    if (!timestamp) return false

    const seenAt = new Date(timestamp)
    if (Number.isNaN(seenAt.getTime())) return false

    return Date.now() - seenAt.getTime() < 5 * 60 * 1000
  }

  function machineOS(machine) {
    return [machine?.os_name, machine?.os_version].filter(Boolean).join(' ') || machine?.platform || ''
  }
</script>

<section class="vstack gap-4">
  {#if loading}
    <article class="card">
      <p>Loading machine...</p>
    </article>
  {:else}
    <header class="hstack justify-between">
      <div>
        <h1>{machine?.hostname || machine?.Hostname || 'Machine'}</h1>
        <p class="text-light">{machineOS(machine)}</p>
      </div>
      <span class="badge" data-variant={isOnline(machine) ? 'success' : 'danger'}>
        {isOnline(machine) ? 'Online' : 'Offline'}
      </span>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <section class="vstack gap-3">
      <h2>Policy Posture</h2>
      <div class="table">
        <table>
          <thead>
            <tr>
              <th>Policy</th>
              <th>Response</th>
              <th>Checked</th>
              <th>Error</th>
              <th>Resolution</th>
            </tr>
          </thead>
          <tbody>
            {#each policyPosture as policy}
              <tr>
                <td>
                  <strong>{policy.name || policy.title}</strong>
                  {#if policy.description}
                    <p class="text-light">{policy.description}</p>
                  {/if}
                </td>
                <td>
                  <span class="badge" data-variant={policy.response === 'passing' ? 'success' : policy.response === 'failing' ? 'danger' : 'warning'}>
                    {policy.stale ? `${policy.response} stale` : policy.response}
                  </span>
                </td>
                <td>{formatTimestamp(policy.checked_at)}</td>
                <td>{policy.last_error || ''}</td>
                <td>{policy.response === 'failing' ? policy.resolution || '' : ''}</td>
              </tr>
            {:else}
              <tr>
                <td colspan="5" class="align-center text-light">No policies target this machine</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>

    <article class="card">
      <header>
        <h2>Execute Query</h2>
      </header>
      <form onsubmit={(event) => { event.preventDefault(); runQuery() }}>
        <label>
          SQL Query
          <textarea bind:value={queryText} rows="6" placeholder="SELECT * FROM processes LIMIT 10;"></textarea>
        </label>
        <footer class="hstack justify-end mt-4">
          <button type="submit" disabled={executing || !queryText.trim()}>
            {executing ? 'Executing...' : 'Run Query'}
          </button>
        </footer>
      </form>
    </article>

    <section class="vstack gap-4">
      <div class="hstack justify-between">
        <h2>Query History</h2>
        <p class="text-light">Showing <strong>{queryStartResult}</strong> to <strong>{queryEndResult}</strong> of <strong>{queryTotalCount}</strong> results</p>
      </div>
      {#if historyLoading}
        <article class="card align-center text-light">Loading query history...</article>
      {:else}
        {#each queryHistory as query}
          <article class="card">
            <div class="hstack justify-between query-history-header">
              <code class="query-text">{query.query}</code>
              <div class="hstack gap-2 query-history-actions">
                {#if query.status}
                  <span class="badge" data-variant={query.status === 'complete' ? 'success' : query.status === 'error' ? 'danger' : 'warning'}>{query.status}</span>
                {/if}
                <small class="text-light">{formatTimestamp(query.timestamp)}</small>
                <button type="button" class="small outline" data-variant="danger" onclick={() => confirmDeleteQuery(query)} aria-label="Delete query result">Delete</button>
              </div>
            </div>
            {#if query.error}
              <pre class="result-fallback"><code>{query.error}</code></pre>
            {:else}
              {@const result = queryResultView(query)}
              {#if result.type === 'pending'}
                <p class="text-light result-message">Awaiting results...</p>
              {:else if result.type === 'empty'}
                <p class="text-light result-message">{result.message}</p>
              {:else if result.type === 'table'}
                <div class="table query-results-table">
                  <table>
                    <thead>
                      <tr>
                        {#each result.columns as column}
                          <th class="result-header">{column}</th>
                        {/each}
                      </tr>
                    </thead>
                    <tbody>
                      {#each result.rows as row, rowIndex}
                        {@const rowKey = resultRowKey(query, rowIndex)}
                        <tr
                          class="result-row"
                          class:expanded-result-row={expandedResultRowKey === rowKey}
                          data-result-row-key={rowKey}
                          tabindex="0"
                          aria-expanded={expandedResultRowKey === rowKey}
                          title="Click to show full row"
                          onclick={() => toggleResultRowByKey(rowKey)}
                          onkeydown={(event) => handleResultRowKeydown(event, query, rowIndex)}
                        >
                          {#each result.columns as column}
                            <td class="result-cell" title={formatCellValue(row[column])}>
                              <span class="result-cell-content">{formatCellValue(row[column])}</span>
                            </td>
                          {/each}
                        </tr>
                        {#if expandedResultRowKey === rowKey}
                          <tr class="result-row-details">
                            <td colspan={result.columns.length}>
                              <pre class="result-row-json"><code>{formatResults(row)}</code></pre>
                            </td>
                          </tr>
                        {/if}
                      {/each}
                    </tbody>
                  </table>
                </div>
              {:else}
                <pre class="result-fallback"><code>{result.text}</code></pre>
              {/if}
            {/if}
          </article>
        {:else}
          <article class="card align-center text-light">No queries executed yet</article>
        {/each}
      {/if}
      {#if queryTotalCount > 0}
        <footer class="hstack justify-end">
          <OatPagination currentPage={currentQueryPage} pageCount={queryPageCount} disabled={historyLoading} label="Query history pagination" onPageChange={changeQueryPage} />
        </footer>
      {/if}
    </section>
  {/if}
</section>

<ConfirmDialog
  bind:open={deleteDialogOpen}
  title="Delete Query Result"
  message="Are you sure you want to delete this query result? This action cannot be undone."
  confirmingLabel="Deleting..."
  confirming={deletingQuery}
  onConfirm={deleteSelectedQuery}
  onCancel={cancelDeleteQuery}
/>

<style>
  .query-history-header {
    align-items: flex-start;
  }

  .query-text {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .query-results-table {
    margin-top: var(--oat-space-3, 1rem);
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
    background-color: rgb(from var(--muted) r g b / .35);
  }

  .result-row {
    cursor: pointer;
  }

  .result-row:focus-visible {
    outline: 2px solid var(--ring);
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
    margin-top: var(--oat-space-3, 1rem);
  }

  .result-fallback {
    margin-top: var(--oat-space-3, 1rem);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
</style>
