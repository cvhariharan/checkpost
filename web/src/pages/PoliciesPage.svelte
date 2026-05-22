<script>
  import { onMount } from 'svelte'
  import { deletePolicy as apiDeletePolicy, fetchPolicies, fetchPolicyMachines } from '@/api.js'
  import ErrorMessage from '@/components/common/ErrorMessage.svelte'
  import SearchInput from '@/components/common/SearchInput.svelte'
  import PolicyFormDialog from '@/components/policies/PolicyFormDialog.svelte'
  import { link } from '@/routes.js'

  let loadedPolicies = []
  let currentPage = 1
  let pageCount = 1
  let totalCount = 0
  let countPerPage = 10
  let searchTerm = ''
  let error = ''
  let formOpen = false
  let editingPolicy = null
  let deleteDialog
  let selectedPolicy = null
  let isDeleting = false
  let machinePolicy = null
  let machineResponse = 'failing'
  let policyMachines = []
  let machinePage = 1
  let machinePageCount = 1
  let machineTotalCount = 0
  let machinesLoading = false
  const machineCountPerPage = 100

  $: policies = loadedPolicies.filter((policy) => {
    const search = searchTerm.trim().toLowerCase()
    return (
      !search ||
      (policy.name || '').toLowerCase().includes(search) ||
      (policy.description || '').toLowerCase().includes(search) ||
      (policy.query || '').toLowerCase().includes(search)
    )
  })
  $: startResult = loadedPolicies.length === 0 ? 0 : (currentPage - 1) * countPerPage + 1
  $: endResult = Math.min(currentPage * countPerPage, totalCount)

  onMount(loadPolicies)

  async function loadPolicies() {
    error = ''
    try {
      const data = await fetchPolicies({ page: currentPage, countPerPage })
      loadedPolicies = data.policies || []
      pageCount = data.page_count || 1
      totalCount = data.total_count || loadedPolicies.length
    } catch (err) {
      error = err.message || 'Failed to fetch policies'
    }
  }

  async function changePage(page) {
    if (page > 0 && page <= pageCount) {
      currentPage = page
      await loadPolicies()
    }
  }

  function openCreate() {
    editingPolicy = null
    formOpen = true
  }

  function openEdit(policy) {
    editingPolicy = policy
    formOpen = true
  }

  async function handleSaved() {
    formOpen = false
    await loadPolicies()
    if (machinePolicy) {
      const refreshed = loadedPolicies.find((policy) => policy.uuid === machinePolicy.uuid)
      if (refreshed) {
        await openMachines(refreshed, machineResponse, machinePage)
      }
    }
  }

  function confirmDelete(policy) {
    selectedPolicy = policy
    deleteDialog.showModal()
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
      deleteDialog.close()
      selectedPolicy = null
      await loadPolicies()
    } catch (err) {
      error = err.message || 'Failed to delete policy'
    } finally {
      isDeleting = false
    }
  }

  async function openMachines(policy, response, page = 1) {
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
      error = err.message || 'Failed to fetch policy machines'
    } finally {
      machinesLoading = false
    }
  }

  function formatTimestamp(timestamp) {
    if (!timestamp) return ''
    try {
      return new Date(timestamp).toLocaleString()
    } catch {
      return timestamp
    }
  }

  function hostname(machine) {
    return machine.hostname || machine.host_identifier || 'Unknown'
  }

  async function changeMachinePage(page) {
    if (machinePolicy && page > 0 && page <= machinePageCount) {
      await openMachines(machinePolicy, machineResponse, page)
    }
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

  <div class="table">
    <table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Platform</th>
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
              {#if policy.description}
                <p class="text-light">{policy.description}</p>
              {/if}
            </td>
            <td><span class="badge outline">{policy.platform}</span></td>
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
              <div class="hstack justify-end gap-2">
                <button type="button" class="small outline" onclick={() => openEdit(policy)}>Edit</button>
                <button type="button" class="small outline" data-variant="danger" onclick={() => confirmDelete(policy)}>Delete</button>
              </div>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="8" class="align-center text-light">No policies found</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>

  <footer class="hstack justify-between">
    <p class="text-light">Showing <strong>{startResult}</strong> to <strong>{endResult}</strong> of <strong>{totalCount}</strong> results</p>
    <nav class="hstack gap-2" aria-label="Pagination">
      <button type="button" class="small outline" disabled={currentPage === 1} onclick={() => changePage(currentPage - 1)}>Previous</button>
      {#each Array.from({ length: pageCount }, (_, index) => index + 1) as page}
        <button type="button" class="small" class:outline={currentPage !== page} onclick={() => changePage(page)}>{page}</button>
      {/each}
      <button type="button" class="small outline" disabled={currentPage === pageCount} onclick={() => changePage(currentPage + 1)}>Next</button>
    </nav>
  </footer>

  {#if machinePolicy}
    <section class="vstack gap-3">
      <header class="hstack justify-between">
        <div>
          <h2>{machinePolicy.name} machines</h2>
          <p class="text-light">Filtered by {machineResponse}</p>
        </div>
        <div class="hstack gap-2">
          {#each ['passing', 'failing', 'unknown'] as response}
            <button type="button" class="small" class:outline={machineResponse !== response} onclick={() => openMachines(machinePolicy, response)}>
              {response}
            </button>
          {/each}
        </div>
      </header>

      <div class="table">
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
              <tr>
                <td colspan="6" class="align-center text-light">Loading machines...</td>
              </tr>
            {:else}
              {#each policyMachines as machine}
                <tr>
                  <td>{hostname(machine)}</td>
                  <td>{machine.platform || 'Unknown'}</td>
                  <td>
                    <span class="badge" data-variant={machine.response === 'passing' ? 'success' : machine.response === 'failing' ? 'danger' : 'warning'}>
                      {machine.stale ? `${machine.response} stale` : machine.response}
                    </span>
                  </td>
                  <td>{formatTimestamp(machine.checked_at)}</td>
                  <td>{machine.last_error || ''}</td>
                  <td class="align-right">
                    <a href={`/machines/${machine.uuid}/query`} use:link={`/machines/${machine.uuid}/query`} class="button small outline">Open</a>
                  </td>
                </tr>
              {:else}
                <tr>
                  <td colspan="6" class="align-center text-light">No machines for this response</td>
                </tr>
              {/each}
            {/if}
          </tbody>
        </table>
      </div>

      {#if machinePageCount > 1}
        <footer class="hstack justify-end gap-2">
          <span class="text-light">{machineTotalCount} machines</span>
          <button type="button" class="small outline" disabled={machinePage === 1} onclick={() => changeMachinePage(machinePage - 1)}>Previous</button>
          <button type="button" class="small outline" disabled={machinePage === machinePageCount} onclick={() => changeMachinePage(machinePage + 1)}>Next</button>
        </footer>
      {/if}
    </section>
  {/if}
</section>

<PolicyFormDialog
  open={formOpen}
  policy={editingPolicy}
  onClose={() => (formOpen = false)}
  onSaved={handleSaved}
/>

<dialog bind:this={deleteDialog} closedby="any">
  <form method="dialog">
    <header>
      <h2>Delete Policy</h2>
      <p>Are you sure you want to delete this policy? This action cannot be undone.</p>
    </header>
    <footer>
      <button type="button" class="outline" onclick={() => deleteDialog.close()}>Cancel</button>
      <button type="button" data-variant="danger" disabled={isDeleting} onclick={deletePolicy}>
        {isDeleting ? 'Deleting...' : 'Delete'}
      </button>
    </footer>
  </form>
</dialog>
