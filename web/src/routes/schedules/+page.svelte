<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import {
    deleteSchedule as apiDeleteSchedule,
    fetchSchedules,
    type Schedule
  } from '$lib/api'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import SearchInput from '$lib/components/SearchInput.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import ScheduleFormDialog from '$lib/components/ScheduleFormDialog.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'
  import ActionsMenu from '$lib/components/ActionsMenu.svelte'
  import Truncate from '$lib/components/Truncate.svelte'
  import { canFrom, me } from '$lib/auth'

  let loadedSchedules = $state<Schedule[]>([])
  let currentPage = $state(1)
  let pageCount = $state(1)
  let totalCount = $state(0)
  const countPerPage = 10
  let searchTerm = $state('')
  let error = $state('')
  let loading = $state(true)
  let formOpen = $state(false)
  let editingSchedule = $state<Schedule | null>(null)
  let deleteOpen = $state(false)
  let selectedSchedule = $state<Schedule | null>(null)
  let isDeleting = $state(false)

  const schedules = $derived(loadedSchedules.filter((s) => {
    const search = searchTerm.trim().toLowerCase()
    return (
      !search ||
      (s.title || '').toLowerCase().includes(search) ||
      (s.sql || '').toLowerCase().includes(search) ||
      String(s.interval || '').includes(search)
    )
  }))
  const startResult = $derived(loadedSchedules.length === 0 ? 0 : (currentPage - 1) * countPerPage + 1)
  const endResult = $derived(Math.min(currentPage * countPerPage, totalCount))
  const canCreateSchedule = $derived(canFrom($me, 'schedule', 'create'))
  const canUpdateSchedule = $derived(canFrom($me, 'schedule', 'update'))
  const canDeleteSchedule = $derived(canFrom($me, 'schedule', 'delete'))

  onMount(loadSchedules)

  async function loadSchedules() {
    loading = true
    error = ''
    try {
      const data = await fetchSchedules({ page: currentPage, countPerPage })
      loadedSchedules = data.schedules || []
      pageCount = data.page_count || 1
      totalCount = data.total_count || loadedSchedules.length
    } catch (err) {
      error = (err as Error).message || 'Failed to fetch schedules'
    } finally {
      loading = false
    }
  }

  async function changePage(page: number) {
    if (page > 0 && page <= pageCount) {
      currentPage = page
      await loadSchedules()
    }
  }

  function openCreate() {
    if (!canCreateSchedule) return
    editingSchedule = null
    formOpen = true
  }

  function openEdit(schedule: Schedule) {
    if (!canUpdateSchedule) return
    editingSchedule = schedule
    formOpen = true
  }

  async function handleSaved() {
    formOpen = false
    await loadSchedules()
  }

  function confirmDelete(schedule: Schedule) {
    if (!canDeleteSchedule) return
    selectedSchedule = schedule
    deleteOpen = true
  }

  async function deleteSchedule() {
    if (!selectedSchedule) return
    isDeleting = true
    error = ''
    try {
      await apiDeleteSchedule(selectedSchedule.uuid)
      deleteOpen = false
      selectedSchedule = null
      await loadSchedules()
    } catch (err) {
      error = (err as Error).message || 'Failed to delete schedule'
    } finally {
      isDeleting = false
    }
  }

  function descriptionFor(schedule: Schedule): string {
    return schedule.description || schedule.sql || ''
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">Query Schedules</h1>
      <p class="text-light">Schedule queries to run on specific machines</p>
    </div>
    {#if canCreateSchedule}
      <button type="button" onclick={openCreate}>Create Schedule</button>
    {/if}
  </header>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  <div class="row">
    <div class="col-6">
      <SearchInput bind:value={searchTerm} placeholder="Search schedules..." />
    </div>
  </div>

  {#if loading}
    <Spinner />
  {:else}
    <div class="table">
      <table class="schedules-table">
        <thead>
          <tr>
            <th class="col-title">Title</th>
            <th>Description</th>
            <th class="col-actions"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {#each schedules as schedule}
            <tr>
              <td>
                {#if canUpdateSchedule}
                  <button type="button" class="cell-link" onclick={() => openEdit(schedule)}>
                    <Truncate text={schedule.title || 'Untitled'} />
                  </button>
                {:else}
                  <strong><Truncate text={schedule.title || 'Untitled'} /></strong>
                {/if}
              </td>
              <td class="text-light">
                <Truncate text={descriptionFor(schedule)} lines={2} />
              </td>
              <td class="col-actions">
                <ActionsMenu label={`Actions for ${schedule.title || 'schedule'}`}>
                  <button role="menuitem" type="button" onclick={() => goto(`/schedules/${schedule.uuid}`)}>Results</button>
                  {#if canUpdateSchedule || canDeleteSchedule}<hr />{/if}
                  {#if canUpdateSchedule}
                    <button role="menuitem" type="button" onclick={() => openEdit(schedule)}>Edit</button>
                  {/if}
                  {#if canUpdateSchedule && canDeleteSchedule}<hr />{/if}
                  {#if canDeleteSchedule}
                    <button role="menuitem" type="button" onclick={() => confirmDelete(schedule)}>Delete</button>
                  {/if}
                </ActionsMenu>
              </td>
            </tr>
          {:else}
            <tr><td colspan="3" class="align-center text-light">No schedules found</td></tr>
          {/each}
        </tbody>
      </table>
    </div>

    <footer class="hstack justify-between">
      <p class="text-light">
        Showing <strong>{startResult}</strong> to <strong>{endResult}</strong> of
        <strong>{totalCount}</strong> results
      </p>
      <Pagination {currentPage} {pageCount} onPageChange={changePage} />
    </footer>
  {/if}
</section>

<ScheduleFormDialog
  open={formOpen}
  schedule={editingSchedule}
  onClose={() => (formOpen = false)}
  onSaved={handleSaved}
/>

<ConfirmDialog
  bind:open={deleteOpen}
  title="Delete Schedule"
  message="Are you sure you want to delete this schedule? This action cannot be undone."
  confirming={isDeleting}
  confirmingLabel="Deleting..."
  onConfirm={deleteSchedule}
  onCancel={() => (selectedSchedule = null)}
/>

<style>
  .schedules-table {
    table-layout: fixed;
    width: 100%;
  }
  .schedules-table .col-title {
    width: 28%;
  }
  .schedules-table .col-actions {
    width: 3rem;
    text-align: right;
  }
</style>
