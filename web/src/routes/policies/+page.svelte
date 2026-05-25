<script lang="ts">
  import { onMount } from 'svelte'
  import {
    deletePolicy as apiDeletePolicy,
    fetchPolicies,
    fetchPolicyMachines,
    type Policy,
    type PolicyMachineRow
  } from '$lib/api'
  import { formatTimestamp, machineHostname } from '$lib/util'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import SearchInput from '$lib/components/SearchInput.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import PolicyFormDialog from '$lib/components/PolicyFormDialog.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'

  let loadedPolicies: Policy[] = []
  let currentPage = 1
  let pageCount = 1
  let totalCount = 0
  const countPerPage = 10
  let searchTerm = ''
  let error = ''
  let loading = true
  let formOpen = false
  let editingPolicy: Policy | null = null
  let deleteOpen = false
  let selectedPolicy: Policy | null = null
  let isDeleting = false
  let machinePolicy: Policy | null = null
  let machineResponse = 'failing'
  let policyMachines: PolicyMachineRow[] = []
  let machinePage = 1
  let machinePageCount = 1
  let machineTotalCount = 0
  let machinesLoading = false
  const machineCountPerPage = 100

  $: policies = loadedPolicies.filter((p) => {
    const search = searchTerm.trim().toLowerCase()
    return (
      !search ||
      (p.name || '').toLowerCase().includes(search) ||
      (p.description || '').toLowerCase().includes(search) ||
      (p.query || '').toLowerCase().includes(search)
    )
  })
  $: startResult = loadedPolicies.length === 0 ? 0 : (currentPage - 1) * countPerPage + 1
  $: endResult = Math.min(currentPage * countPerPage, totalCount)

  onMount(loadPolicies)

  async function loadPolicies() {
    loading = true
    error = ''
    try {
      const data = await fetchPolicies({ page: currentPage, countPerPage })
      loadedPolicies = data.policies || []
      pageCount = data.page_count || 1
      totalCount = data.total_count || loadedPolicies.length
    } catch (err) {
      error = (err as Error).message || 'Failed to fetch policies'
    } finally {
      loading = false
    }
  }

  async function changePage(page: number) {
    if (page > 0 && page <= pageCount) {
      currentPage = page
      await loadPolicies()
    }
  }

  function openCreate() {
    editingPolicy = null
    formOpen = true
  }

  function openEdit(policy: Policy) {
    editingPolicy = policy
    formOpen = true
  }

  async function handleSaved() {
    formOpen = false
    await loadPolicies()
    if (machinePolicy) {
      const refreshed = loadedPolicies.find((p) => p.uuid === machinePolicy!.uuid)
      if (refreshed) await openMachines(refreshed, machineResponse, machinePage)
    }
  }

  function confirmDelete(policy: Policy) {
    selectedPolicy = policy
    deleteOpen = true
  }

  async function deletePolicy() {
    if (!selectedPolicy) return
    isDeleting = true
    error = ''
    try {
      await apiDeletePolicy(selectedPolicy.uuid)
      if (machinePolicy?.uuid === selectedPolicy.uuid) {
        machinePolicy = null
        policyMachines = []
        machinePage = 1
        machinePageCount = 1
        machineTotalCount = 0
      }
      deleteOpen = false
      selectedPolicy = null
      await loadPolicies()
    } catch (err) {
      error = (err as Error).message || 'Failed to delete policy'
    } finally {
      isDeleting = false
    }
  }

  async function openMachines(policy: Policy, response: string, page = 1) {
    machinePolicy = policy
    machineResponse = response
    machinePage = page
    machinesLoading = true
    error = ''
    try {
      const data = await fetchPolicyMachines(policy.uuid, { response, page, countPerPage: machineCountPerPage })
      policyMachines = data.machines || []
      machinePageCount = data.page_count || 1
      machineTotalCount = data.total_count || policyMachines.length
    } catch (err) {
      error = (err as Error).message || 'Failed to fetch policy machines'
    } finally {
      machinesLoading = false
    }
  }

  function targetLabel(policy: Policy): string {
    if (policy.target_all_machines || !policy.groups?.length) return 'All machines'
    return policy.groups.map((g) => g.name).join(', ')
  }

  async function changeMachinePage(page: number) {
    if (machinePolicy && page > 0 && page <= machinePageCount) {
      await openMachines(machinePolicy, machineResponse, page)
    }
  }

  function responseVariant(response?: string): 'success' | 'danger' | 'warning' {
    if (response === 'passing') return 'success'
    if (response === 'failing') return 'danger'
    return 'warning'
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between">
    <div>
      <h1>Policies</h1>
      <p class="text-light">Evaluate osquery-backed posture checks across enrolled machines</p>
    </div>
    <button type="button" onclick={openCreate}>Create Policy</button>
  </header>

  <div class="row">
    <div class="col-6">
      <SearchInput bind:value={searchTerm} placeholder="Search policies..." />
    </div>
  </div>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if loading}
    <Spinner />
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
            <th class="align-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each policies as policy}
            <tr>
              <td>
                <strong>{policy.name || policy.title || 'Untitled'}</strong>
                {#if policy.description}<p class="text-light">{policy.description}</p>{/if}
              </td>
              <td><span class="badge outline">{policy.platform}</span></td>
              <td>{targetLabel(policy)}</td>
              <td>
                <span class="badge" data-variant={policy.enabled ? 'success' : 'warning'}>
                  {policy.enabled ? 'Enabled' : 'Disabled'}
                </span>
              </td>
              <td class="align-right">
                <button type="button" class="small outline" onclick={() => openMachines(policy, 'passing')}>
                  {policy.passing_count || 0}
                </button>
              </td>
              <td class="align-right">
                <button type="button" class="small outline" onclick={() => openMachines(policy, 'failing')}>
                  {policy.failing_count || 0}
                </button>
              </td>
              <td class="align-right">
                <button type="button" class="small outline" onclick={() => openMachines(policy, 'unknown')}>
                  {policy.unknown_count || 0}
                </button>
              </td>
              <td>{formatTimestamp(policy.last_count_updated_at)}</td>
              <td class="align-right">
                <menu class="buttons">
                  <li><button type="button" class="small outline" onclick={() => openEdit(policy)}>Edit</button></li>
                  <li>
                    <button
                      type="button"
                      class="small outline"
                      data-variant="danger"
                      onclick={() => confirmDelete(policy)}
                    >
                      Delete
                    </button>
                  </li>
                </menu>
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
      <Pagination {currentPage} {pageCount} onPageChange={changePage} />
    </footer>

    {#if machinePolicy}
      <section class="vstack gap-3">
        <header class="hstack justify-between">
          <div>
            <h2>{machinePolicy.name} machines</h2>
            <p class="text-light">Filtered by {machineResponse}</p>
          </div>
          <menu class="buttons">
            {#each ['passing', 'failing', 'unknown'] as response}
              <li>
                <button
                  type="button"
                  class={machineResponse === response ? 'small' : 'small outline'}
                  aria-current={machineResponse === response ? 'true' : undefined}
                  onclick={() => openMachines(machinePolicy!, response)}
                >
                  {response}
                </button>
              </li>
            {/each}
          </menu>
        </header>

        <div class="table" aria-busy={machinesLoading ? 'true' : undefined}>
          <table>
            <thead>
              <tr>
                <th>Machine</th>
                <th>Platform</th>
                <th>Response</th>
                <th>Checked</th>
                <th>Error</th>
                <th class="align-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {#if machinesLoading}
                <tr><td colspan="6" class="align-center text-light">Loading machines...</td></tr>
              {:else}
                {#each policyMachines as machine}
                  <tr>
                    <td>{machineHostname(machine as any)}</td>
                    <td>{machine.platform || 'Unknown'}</td>
                    <td>
                      <span class="badge" data-variant={responseVariant(machine.response)}>
                        {machine.stale ? `${machine.response} stale` : machine.response}
                      </span>
                    </td>
                    <td>{formatTimestamp(machine.checked_at)}</td>
                    <td>{machine.last_error || ''}</td>
                    <td class="align-right">
                      <a href="/machines/{machine.uuid}" class="button small outline">Open</a>
                    </td>
                  </tr>
                {:else}
                  <tr><td colspan="6" class="align-center text-light">No machines for this response</td></tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>

        {#if machinePageCount > 1}
          <footer class="hstack justify-between">
            <span class="text-light">{machineTotalCount} machines</span>
            <Pagination
              currentPage={machinePage}
              pageCount={machinePageCount}
              disabled={machinesLoading}
              label="Policy machines pagination"
              onPageChange={changeMachinePage}
            />
          </footer>
        {/if}
      </section>
    {/if}
  {/if}
</section>

<PolicyFormDialog
  open={formOpen}
  policy={editingPolicy}
  onClose={() => (formOpen = false)}
  onSaved={handleSaved}
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
