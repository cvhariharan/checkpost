<script lang="ts">
  import { onMount } from 'svelte'
  import { page } from '$app/state'
  import {
    fetchSchedule,
    fetchScheduleResults,
    type Schedule,
    type ScheduleResultRow
  } from '$lib/api'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import ScheduleResultsTable from '$lib/components/ScheduleResultsTable.svelte'
  import Spinner from '$lib/components/Spinner.svelte'

  let schedule = $state<Schedule | null>(null)
  let columns = $state<string[]>([])
  let rows = $state<ScheduleResultRow[]>([])
  let total = $state(0)
  let currentPage = $state(1)
  let pageCount = $state(1)
  let query = $state('')
  let lastRefreshed = $state('')
  let loading = $state(true)
  let resultsLoading = $state(false)
  let error = $state('')
  let browsingDisabled = $state(false)
  const countPerPage = 500

  const scheduleId = $derived(page.params.id as string)

  onMount(loadAll)

  async function loadAll() {
    loading = true
    error = ''
    try {
      schedule = await fetchSchedule(scheduleId)
      await loadResults(1, '')
    } catch (err) {
      error = (err as Error).message || 'Failed to load schedule'
    } finally {
      loading = false
    }
  }

  async function loadResults(targetPage = currentPage, q = query) {
    resultsLoading = true
    error = ''
    try {
      const data = await fetchScheduleResults(scheduleId, { page: targetPage, countPerPage, query: q })
      browsingDisabled = data.browsing_disabled || false
      columns = data.columns || []
      rows = data.rows || []
      total = data.total || 0
      currentPage = data.page || targetPage
      pageCount = data.page_count || Math.max(1, Math.ceil(total / countPerPage))
      query = q
      lastRefreshed = new Date().toLocaleTimeString()
    } catch (err) {
      error = (err as Error).message || 'Failed to load results'
    } finally {
      resultsLoading = false
    }
  }

  function refresh() {
    loadResults(currentPage, query)
  }

  function applyQuery(q: string) {
    loadResults(1, q)
  }

  function changePage(p: number) {
    if (p > 0 && p <= pageCount) loadResults(p, query)
  }
</script>

<ErrorMessage message={error} onClose={() => (error = '')} />

{#if loading}
  <Spinner fill />
{:else if schedule}
  {#if browsingDisabled}
    <div role="alert" data-variant="warning">
      <p>Result browsing is disabled on this server. Contact an administrator.</p>
    </div>
  {/if}
  <ScheduleResultsTable
    {schedule}
    {columns}
    {rows}
    {total}
    page={currentPage}
    {pageCount}
    {countPerPage}
    loading={resultsLoading}
    {query}
    {lastRefreshed}
    onRefresh={refresh}
    onApplyQuery={applyQuery}
    onPageChange={changePage}
  />
{/if}
