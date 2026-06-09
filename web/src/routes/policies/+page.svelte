<script lang="ts">
  import { untrack } from 'svelte'
  import { page } from '$app/state'
  import { replaceState } from '$app/navigation'
  import { deletePolicy as apiDeletePolicy, fetchPolicies, type Policy } from '$lib/api'
  import { formatTimestamp } from '$lib/util'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import SearchInput from '$lib/components/SearchInput.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import PolicyFormDialog from '$lib/components/PolicyFormDialog.svelte'
  import PolicyMachinesDialog from '$lib/components/PolicyMachinesDialog.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'
  import ActionsMenu from '$lib/components/ActionsMenu.svelte'
  import { canFrom, me } from '$lib/auth'

  let loadedPolicies = $state<Policy[]>([])
  const currentPage = $derived(Math.max(1, Number(page.url.searchParams.get('page')) || 1))
  let pageCount = $state(1)
  let totalCount = $state(0)
  const countPerPage = 10
  let searchTerm = $state('')
  let error = $state('')
  let loading = $state(true)
  let formOpen = $state(false)
  let editingPolicy = $state<Policy | null>(null)
  let deleteOpen = $state(false)
  let selectedPolicy = $state<Policy | null>(null)
  let isDeleting = $state(false)
  let machinesPolicy = $state<Policy | null>(null)
  let machinesOpen = $state(false)

  let initialized = $state(false)
  let previousSearch = ''
  let searchTimer: ReturnType<typeof setTimeout> | undefined

  $effect(() => {
    if (!initialized || searchTerm === previousSearch) return
    previousSearch = searchTerm
    if (currentPage !== 1) {
      const url = new URL(page.url)
      url.searchParams.delete('page')
      replaceState(url, {})
    }
    clearTimeout(searchTimer)
    searchTimer = setTimeout(() => void loadPolicies(), 250)
  })

  const startResult = $derived(loadedPolicies.length === 0 ? 0 : (currentPage - 1) * countPerPage + 1)
  const endResult = $derived(Math.min(currentPage * countPerPage, totalCount))
  const canCreatePolicy = $derived(canFrom($me, 'policy', 'create'))
  const canUpdatePolicy = $derived(canFrom($me, 'policy', 'update'))
  const canDeletePolicy = $derived(canFrom($me, 'policy', 'delete'))

  $effect(() => {
    currentPage
    untrack(() => void loadPolicies())
  })

  async function loadPolicies() {
    loading = true
    error = ''
    try {
      const data = await fetchPolicies({ page: currentPage, countPerPage, query: searchTerm })
      loadedPolicies = data.policies || []
      pageCount = data.page_count || 1
      totalCount = data.total_count || loadedPolicies.length
    } catch (err) {
      error = (err as Error).message || 'Failed to fetch policies'
    } finally {
      previousSearch = searchTerm
      initialized = true
      loading = false
    }
  }

  function openCreate() {
    if (!canCreatePolicy) return
    editingPolicy = null
    formOpen = true
  }

  function openEdit(policy: Policy) {
    if (!canUpdatePolicy) return
    editingPolicy = policy
    formOpen = true
  }

  async function handleSaved() {
    formOpen = false
    await loadPolicies()
  }

  function confirmDelete(policy: Policy) {
    if (!canDeletePolicy) return
    selectedPolicy = policy
    deleteOpen = true
  }

  async function deletePolicy() {
    if (!selectedPolicy) return
    isDeleting = true
    error = ''
    try {
      await apiDeletePolicy(selectedPolicy.uuid)
      deleteOpen = false
      selectedPolicy = null
      await loadPolicies()
    } catch (err) {
      error = (err as Error).message || 'Failed to delete policy'
    } finally {
      isDeleting = false
    }
  }

  function targetLabel(policy: Policy): string {
    if (policy.target_all_machines || !policy.groups?.length) return 'All machines'
    return policy.groups.map((g) => g.name).join(', ')
  }

  function openMachinesModal(policy: Policy) {
    machinesPolicy = policy
    machinesOpen = true
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">Policies</h1>
      <p class="text-light">Evaluate osquery-backed posture checks across enrolled machines</p>
    </div>
    {#if canCreatePolicy}
      <button type="button" onclick={openCreate}>Create Policy</button>
    {/if}
  </header>

  <div class="row">
    <div class="col-6">
      <SearchInput bind:value={searchTerm} placeholder="Search policies..." />
    </div>
  </div>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if loading}
    <Spinner fill />
  {:else}
    <div class="table">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Platform</th>
            <th>Targets</th>
            <th>Status</th>
            <th class="align-right">Passing</th>
            <th class="align-right">Failing</th>
            <th class="align-right">Unknown</th>
            <th>Updated</th>
            <th class="align-right"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {#each loadedPolicies as policy}
            <tr>
              <td>
                <button type="button" class="cell-link" onclick={() => openMachinesModal(policy)}>
                  {policy.name || policy.title || 'Untitled'}
                </button>
                {#if policy.description}<p class="text-light">{policy.description}</p>{/if}
              </td>
              <td><span class="badge outline">{policy.platform}</span></td>
              <td>{targetLabel(policy)}</td>
              <td>
                <span class="badge" data-variant={policy.enabled ? 'success' : 'warning'}>
                  {policy.enabled ? 'Enabled' : 'Disabled'}
                </span>
              </td>
              <td class="align-right">{policy.passing_count || 0}</td>
              <td class="align-right">{policy.failing_count || 0}</td>
              <td class="align-right">{policy.unknown_count || 0}</td>
              <td>{formatTimestamp(policy.last_count_updated_at)}</td>
              <td class="align-right">
                {#if canUpdatePolicy || canDeletePolicy}
                  <ActionsMenu label={`Actions for ${policy.name || policy.title || 'policy'}`}>
                    {#if canUpdatePolicy}
                      <button role="menuitem" type="button" onclick={() => openEdit(policy)}>Edit</button>
                    {/if}
                    {#if canUpdatePolicy && canDeletePolicy}<hr />{/if}
                    {#if canDeletePolicy}
                      <button role="menuitem" type="button" onclick={() => confirmDelete(policy)}>Delete</button>
                    {/if}
                  </ActionsMenu>
                {/if}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="9" class="align-center text-light">No policies found</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <footer class="hstack justify-between">
      <p class="text-light">
        Showing <strong>{startResult}</strong> to <strong>{endResult}</strong> of
        <strong>{totalCount}</strong> results
      </p>
      <Pagination {currentPage} {pageCount} param="page" />
    </footer>
  {/if}
</section>

<PolicyFormDialog
  open={formOpen}
  policy={editingPolicy}
  onClose={() => (formOpen = false)}
  onSaved={handleSaved}
/>

<PolicyMachinesDialog
  bind:open={machinesOpen}
  policy={machinesPolicy}
  onClose={() => (machinesPolicy = null)}
/>

<ConfirmDialog
  bind:open={deleteOpen}
  title="Delete Policy"
  message="Are you sure you want to delete this policy? This action cannot be undone."
  confirming={isDeleting}
  confirmingLabel="Deleting..."
  onConfirm={deletePolicy}
  onCancel={() => (selectedPolicy = null)}
/>
