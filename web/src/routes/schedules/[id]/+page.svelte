<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import { page } from '$app/stores'
  import {
    fetchSchedule,
    fetchScheduleResults,
    type Schedule,
    type ScheduleResultRow
  } from '$lib/api'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import ScheduleResultsTable from '$lib/components/ScheduleResultsTable.svelte'
  import Spinner from '$lib/components/Spinner.svelte'

  let schedule: Schedule | null = null
  let columns: string[] = []
  let rows: ScheduleResultRow[] = []
  let total = 0
  let currentPage = 1
  let pageCount = 1
  let query = ''
  let lastRefreshed = ''
  let loading = true
  let resultsLoading = false
  let error = ''
  const countPerPage = 500

  $: scheduleId = $page.params.id as string

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

  function close() {
    goto('/schedules')
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
  <Spinner />
{:else if schedule}
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
    onClose={close}
    onRefresh={refresh}
    onApplyQuery={applyQuery}
    onPageChange={changePage}
  />
{/if}
