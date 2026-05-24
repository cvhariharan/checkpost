<script>
  import { onMount } from 'svelte'
  import { deleteSchedule as apiDeleteSchedule, fetchScheduleResults, fetchSchedules } from '@/api.js'
  import ErrorMessage from '@/components/common/ErrorMessage.svelte'
  import OatPagination from '@/components/common/OatPagination.svelte'
  import SearchInput from '@/components/common/SearchInput.svelte'
  import ScheduleFormDialog from '@/components/schedules/ScheduleFormDialog.svelte'
  import ScheduleResultsTable from '@/components/schedules/ScheduleResultsTable.svelte'

  let loadedSchedules = []
  let currentPage = 1
  let pageCount = 1
  let totalCount = 0
  let countPerPage = 10
  let searchTerm = ''
  let error = ''
  let formOpen = false
  let editingSchedule = null
  let deleteDialog
  let selectedSchedule = null
  let isDeleting = false

  let resultsSchedule = null
  let resultsColumns = []
  let resultsRows = []
  let resultsTotal = 0
  let resultsPage = 1
  let resultsPageCount = 1
  let resultsQuery = ''
  let resultsLastRefreshed = ''
  let resultsLoading = false
  const resultsPerPage = 500

  $: schedules = loadedSchedules.filter((schedule) => {
    const search = searchTerm.trim().toLowerCase()
    return (
      !search ||
      (schedule.title || '').toLowerCase().includes(search) ||
      (schedule.query?.query || '').toLowerCase().includes(search) ||
      String(schedule.interval || '').includes(search)
    )
  })
  $: startResult = loadedSchedules.length === 0 ? 0 : (currentPage - 1) * countPerPage + 1
  $: endResult = Math.min(currentPage * countPerPage, totalCount)
  $: scheduleListHidden = !!resultsSchedule

  onMount(loadSchedules)

  async function loadSchedules() {
    error = ''
    try {
      const data = await fetchSchedules({ page: currentPage, countPerPage })
      loadedSchedules = data.schedules || []
      pageCount = data.page_count || 1
      totalCount = data.total_count || loadedSchedules.length
    } catch (err) {
      error = err.message || 'Failed to fetch schedules'
    }
  }

  async function changePage(page) {
    if (page > 0 && page <= pageCount) {
      currentPage = page
      await loadSchedules()
    }
  }

  function openCreate() {
    editingSchedule = null
    formOpen = true
  }

  function openEdit(schedule) {
    editingSchedule = schedule
    formOpen = true
  }

  async function handleSaved() {
    formOpen = false
    await loadSchedules()
    if (resultsSchedule) {
      const refreshed = loadedSchedules.find((s) => s.uuid === resultsSchedule.uuid)
      if (refreshed) {
        await openResults(refreshed, resultsPage, resultsQuery)
      }
    }
  }

  function confirmDelete(schedule) {
    selectedSchedule = schedule
    deleteDialog.showModal()
  }

  async function deleteSchedule() {
    if (!selectedSchedule) return
    isDeleting = true
    error = ''
    try {
      await apiDeleteSchedule(selectedSchedule.uuid)
      if (resultsSchedule?.uuid === selectedSchedule.uuid) {
        closeResults()
      }
      deleteDialog.close()
      selectedSchedule = null
      await loadSchedules()
    } catch (err) {
      error = err.message || 'Failed to delete schedule'
    } finally {
      isDeleting = false
    }
  }

  async function openResults(schedule, page = 1, query = resultsQuery) {
    resultsSchedule = schedule
    resultsPage = page
    resultsQuery = query
    resultsLoading = true
    error = ''
    try {
      const data = await fetchScheduleResults(schedule.uuid, { page, countPerPage: resultsPerPage, query })
      resultsColumns = data.columns || []
      resultsRows = data.rows || []
      resultsTotal = data.total || 0
      resultsPage = data.page || page
      resultsPageCount = data.page_count || Math.max(1, Math.ceil(resultsTotal / resultsPerPage))
      resultsLastRefreshed = new Date().toLocaleTimeString()
    } catch (err) {
      error = err.message || 'Failed to load results'
    } finally {
      resultsLoading = false
    }
  }

  function closeResults() {
    resultsSchedule = null
    resultsColumns = []
    resultsRows = []
    resultsTotal = 0
    resultsPage = 1
    resultsPageCount = 1
    resultsQuery = ''
    resultsLastRefreshed = ''
  }

  async function changeResultsPage(page) {
    if (resultsSchedule && page > 0 && page <= resultsPageCount) {
      await openResults(resultsSchedule, page)
    }
  }

  async function applyResultsQuery(query) {
    if (resultsSchedule) {
      await openResults(resultsSchedule, 1, query)
    }
  }

  async function refreshResults() {
    if (resultsSchedule) {
      await openResults(resultsSchedule, resultsPage, resultsQuery)
    }
  }

  function targetLabel(schedule) {
    if (schedule.target_all_machines || !schedule.groups?.length) {
      return 'All machines'
    }
    return schedule.groups.map((group) => group.name).join(', ')
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between">
    <div>
      <h1>Query Schedules</h1>
      <p class="text-light">Schedule queries to run on specific machines</p>
    </div>
    <button type="button" onclick={openCreate}>Create Schedule</button>
  </header>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if scheduleListHidden}
    <ScheduleResultsTable
      schedule={resultsSchedule}
      columns={resultsColumns}
      rows={resultsRows}
      total={resultsTotal}
      page={resultsPage}
      pageCount={resultsPageCount}
      countPerPage={resultsPerPage}
      loading={resultsLoading}
      query={resultsQuery}
      lastRefreshed={resultsLastRefreshed}
      onClose={closeResults}
      onRefresh={refreshResults}
      onApplyQuery={applyResultsQuery}
      onPageChange={changeResultsPage}
    />
  {:else}
    <div class="row">
      <div class="col-6">
        <SearchInput bind:value={searchTerm} placeholder="Search schedules..." />
      </div>
    </div>

    <div class="table">
      <table>
        <thead>
          <tr>
            <th>Title</th>
            <th>Frequency</th>
            <th>Targets</th>
            <th class="align-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each schedules as schedule}
            <tr>
              <td>
                <strong>{schedule.title || 'Untitled'}</strong>
                <p class="text-light">{schedule.query?.query || ''}</p>
                {#if schedule.query?.description}
                  <small class="text-lighter">{schedule.query.description}</small>
                {/if}
              </td>
              <td>{schedule.interval}</td>
              <td>{targetLabel(schedule)}</td>
              <td class="align-right">
                <div class="hstack justify-end gap-2">
                  <button type="button" class="small outline" onclick={() => openResults(schedule)}>Results</button>
                  <button type="button" class="small outline" onclick={() => openEdit(schedule)}>Edit</button>
                  <button type="button" class="small outline" data-variant="danger" onclick={() => confirmDelete(schedule)}>Delete</button>
                </div>
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="4" class="align-center text-light">No schedules found</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <footer class="hstack justify-between">
      <p class="text-light">Showing <strong>{startResult}</strong> to <strong>{endResult}</strong> of <strong>{totalCount}</strong> results</p>
      <OatPagination {currentPage} {pageCount} onPageChange={changePage} />
    </footer>
  {/if}
</section>

<ScheduleFormDialog
  open={formOpen}
  schedule={editingSchedule}
  onClose={() => (formOpen = false)}
  onSaved={handleSaved}
/>

<dialog bind:this={deleteDialog} closedby="any">
  <form method="dialog">
    <header>
      <h2>Delete Schedule</h2>
      <p>Are you sure you want to delete this schedule? This action cannot be undone.</p>
    </header>
    <footer>
      <button type="button" class="outline" onclick={() => deleteDialog.close()}>Cancel</button>
      <button type="button" data-variant="danger" disabled={isDeleting} onclick={deleteSchedule}>
        {isDeleting ? 'Deleting...' : 'Delete'}
      </button>
    </footer>
  </form>
</dialog>
