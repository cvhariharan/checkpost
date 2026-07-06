<script lang="ts">
  import { onMount } from 'svelte'
  import { page } from '$app/state'
  import {
    fetchSchedule,
    fetchScheduleResults,
    scheduleResultsExportUrl,
    type Schedule,
    type ScheduleResultRow
  } from '$lib/api'
  import { formatTimestamp } from '$lib/util'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import DownloadResultsButton from '$lib/components/DownloadResultsButton.svelte'
  import QueryResultTable from '$lib/components/QueryResultTable.svelte'
  import Spinner from '$lib/components/Spinner.svelte'

  let schedule = $state<Schedule | null>(null)
  let columns = $state<string[]>([])
  let rows = $state<ScheduleResultRow[]>([])
  let total = $state(0)
  let currentPage = $state(1)
  let pageCount = $state(1)
  let loading = $state(true)
  let resultsLoading = $state(false)
  let error = $state('')
  let browsingDisabled = $state(false)
  let exportSupported = $state(false)
  let filterQuery = $state('')
  const countPerPage = 100

  const scheduleId = $derived(page.params.id as string)
  const downloadHref = $derived(exportSupported && !browsingDisabled ? scheduleResultsExportUrl(scheduleId) : '')

  // Flatten schedule rows into the plain {column: value} shape QueryResultTable
  // expects, prepending machine + last seen so they show as the first columns.
  const tableColumns = $derived(['Machine', 'Last seen', ...columns])
  const tableRows = $derived(
    rows.map((r) => ({
      Machine: r.hostname || r.node_uuid || '',
      'Last seen': formatTimestamp(r.last_seen),
      ...Object.fromEntries(columns.map((c) => [c, String(r.columns?.[c] ?? '')]))
    }))
  )

  onMount(loadAll)

  async function loadAll() {
    loading = true
    error = ''
    try {
      schedule = await fetchSchedule(scheduleId)
      await loadResults(1)
    } catch (err) {
      error = (err as Error).message || 'Failed to load schedule'
    } finally {
      loading = false
    }
  }

  async function loadResults(targetPage = currentPage) {
    resultsLoading = true
    error = ''
    try {
      const data = await fetchScheduleResults(scheduleId, {
        page: targetPage,
        countPerPage,
        query: filterQuery
      })
      browsingDisabled = data.browsing_disabled || false
      exportSupported = data.export_supported || false
      columns = data.columns || []
      rows = data.rows || []
      total = data.total || 0
      currentPage = data.page || targetPage
      pageCount = data.page_count || Math.max(1, Math.ceil(total / countPerPage))
    } catch (err) {
      error = (err as Error).message || 'Failed to load results'
    } finally {
      resultsLoading = false
    }
  }

  function changePage(p: number) {
    if (p > 0 && p <= pageCount) loadResults(p)
  }

  function applyFilter(event: SubmitEvent) {
    event.preventDefault()
    loadResults(1)
  }
</script>

<ErrorMessage message={error} onClose={() => (error = '')} />

{#if loading}
  <Spinner fill />
{:else if schedule}
  <section class="vstack gap-3">
    <header class="hstack justify-between">
      <div>
        <h2>{schedule.title || 'Schedule results'}</h2>
        <p class="text-light">{schedule.description || ''}</p>
      </div>
      <menu class="buttons">
        {#if downloadHref}
          <li><DownloadResultsButton href={downloadHref} /></li>
        {/if}
        <li><button type="button" class="small outline" disabled={resultsLoading} onclick={() => loadResults(currentPage)}>Refresh</button></li>
      </menu>
    </header>

    <form class="vstack gap-1" onsubmit={applyFilter}>
      <fieldset class="group" aria-label="Filter results">
        <input
          type="search"
          bind:value={filterQuery}
          placeholder="Filter results, e.g. machine:web-01 username:root"
          aria-label="Filter results"
          disabled={browsingDisabled}
        />
        <button type="submit" disabled={resultsLoading || browsingDisabled}>Filter</button>
      </fieldset>
      <small data-hint>
        Filter by column (<code>name:value</code>), <code>machine:</code>, or
        <code>last_seen&gt;=</code>/<code>&lt;=</code> with an RFC3339 time. Space-separated terms are ANDed.
      </small>
    </form>

    <QueryResultTable
      columns={tableColumns}
      rows={tableRows}
      {total}
      page={currentPage}
      {pageCount}
      loading={resultsLoading}
      {browsingDisabled}
      onPageChange={changePage}
    />
  </section>
{/if}
