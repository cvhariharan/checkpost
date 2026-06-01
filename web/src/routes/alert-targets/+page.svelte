<script lang="ts">
  import { onMount } from 'svelte'
  import {
    deleteAlertTarget,
    fetchAlertTargets,
    testAlertTarget,
    type AlertTarget
  } from '$lib/api'
  import { formatTimestamp } from '$lib/util'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'
  import ActionsMenu from '$lib/components/ActionsMenu.svelte'
  import AlertTargetFormDialog from '$lib/components/AlertTargetFormDialog.svelte'
  import { canFrom, me } from '$lib/auth'

  let targets = $state<AlertTarget[]>([])
  let currentPage = $state(1)
  let pageCount = $state(1)
  let totalCount = $state(0)
  const countPerPage = 10
  let error = $state('')
  let notice = $state('')
  let loading = $state(true)
  let formOpen = $state(false)
  let editing = $state<AlertTarget | null>(null)
  let deleteOpen = $state(false)
  let selected = $state<AlertTarget | null>(null)
  let isDeleting = $state(false)
  let testingId = $state('')

  const canCreate = $derived(canFrom($me, 'alert_target', 'create'))
  const canUpdate = $derived(canFrom($me, 'alert_target', 'update'))
  const canDelete = $derived(canFrom($me, 'alert_target', 'delete'))
  const canTest = $derived(canFrom($me, 'alert_target', 'execute'))

  onMount(load)

  async function load() {
    loading = true
    error = ''
    try {
      const data = await fetchAlertTargets({ page: currentPage, countPerPage })
      targets = data.targets || []
      pageCount = data.page_count || 1
      totalCount = data.total_count || targets.length
    } catch (err) {
      error = (err as Error).message || 'Failed to fetch alert targets'
    } finally {
      loading = false
    }
  }

  async function changePage(page: number) {
    if (page > 0 && page <= pageCount) {
      currentPage = page
      await load()
    }
  }

  function openCreate() {
    if (!canCreate) return
    editing = null
    formOpen = true
  }

  function openEdit(target: AlertTarget) {
    if (!canUpdate) return
    editing = target
    formOpen = true
  }

  async function handleSaved() {
    formOpen = false
    await load()
  }

  function confirmDelete(target: AlertTarget) {
    if (!canDelete) return
    selected = target
    deleteOpen = true
  }

  async function remove() {
    if (!selected) return
    isDeleting = true
    error = ''
    try {
      await deleteAlertTarget(selected.uuid)
      deleteOpen = false
      selected = null
      await load()
    } catch (err) {
      error = (err as Error).message || 'Failed to delete alert target'
    } finally {
      isDeleting = false
    }
  }

  async function sendTest(target: AlertTarget) {
    testingId = target.uuid
    error = ''
    notice = ''
    try {
      await testAlertTarget(target.uuid)
      notice = `Test alert sent to ${target.name}`
    } catch (err) {
      error = (err as Error).message || 'Test delivery failed'
    } finally {
      testingId = ''
    }
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">Alert Targets</h1>
      <p class="text-light">Where alerts are delivered — webhooks and email</p>
    </div>
    {#if canCreate}
      <button type="button" onclick={openCreate}>Create Target</button>
    {/if}
  </header>

  <ErrorMessage message={error} onClose={() => (error = '')} />
  {#if notice}
    <p class="badge" data-variant="success">{notice}</p>
  {/if}

  {#if loading}
    <Spinner />
  {:else}
    <div class="table">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Status</th>
            <th>Updated</th>
            <th class="align-right"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {#each targets as target}
            <tr>
              <td>
                <button type="button" class="cell-link" onclick={() => openEdit(target)}>
                  {target.name || 'Untitled'}
                </button>
              </td>
              <td><span class="badge outline">{target.type}</span></td>
              <td>
                <span class="badge" data-variant={target.enabled ? 'success' : 'warning'}>
                  {target.enabled ? 'Enabled' : 'Disabled'}
                </span>
              </td>
              <td>{formatTimestamp(target.updated_at)}</td>
              <td class="align-right">
                {#if canUpdate || canDelete || canTest}
                  <ActionsMenu label={`Actions for ${target.name || 'target'}`}>
                    {#if canTest}
                      <button
                        role="menuitem"
                        type="button"
                        disabled={testingId === target.uuid}
                        onclick={() => sendTest(target)}
                      >
                        {testingId === target.uuid ? 'Sending...' : 'Send test'}
                      </button>
                    {/if}
                    {#if canUpdate}
                      <button role="menuitem" type="button" onclick={() => openEdit(target)}>Edit</button>
                    {/if}
                    {#if canDelete}<hr />
                      <button role="menuitem" type="button" onclick={() => confirmDelete(target)}>Delete</button>
                    {/if}
                  </ActionsMenu>
                {/if}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="5" class="align-center text-light">No alert targets found</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <footer class="hstack justify-between">
      <p class="text-light"><strong>{totalCount}</strong> targets</p>
      <Pagination {currentPage} {pageCount} onPageChange={changePage} />
    </footer>
  {/if}
</section>

<AlertTargetFormDialog
  open={formOpen}
  target={editing}
  onClose={() => (formOpen = false)}
  onSaved={handleSaved}
/>

<ConfirmDialog
  bind:open={deleteOpen}
  title="Delete Alert Target"
  message="Are you sure you want to delete this alert target? This action cannot be undone."
  confirming={isDeleting}
  confirmingLabel="Deleting..."
  onConfirm={remove}
  onCancel={() => (selected = null)}
/>
