<script lang="ts">
  import { fetchPolicyMachines, type Policy, type PolicyMachineRow } from '$lib/api'
  import { formatTimestamp, machineHostname } from '$lib/util'
  import Pagination from './Pagination.svelte'
  import ErrorMessage from './ErrorMessage.svelte'

  export let open = false
  export let policy: Policy | null = null
  export let onClose: () => void = () => {}

  const countPerPage = 100
  const tabs: { value: string; label: string }[] = [
    { value: '', label: 'All' },
    { value: 'passing', label: 'Passing' },
    { value: 'failing', label: 'Failing' },
    { value: 'unknown', label: 'Unknown' }
  ]

  let dialog: HTMLDialogElement
  let preparedFor: string | null = null
  let response = ''
  let machines: PolicyMachineRow[] = []
  let page = 1
  let pageCount = 1
  let totalCount = 0
  let loading = false
  let error = ''

  $: if (open && dialog) {
    const key = policy?.uuid || ''
    if (preparedFor !== key) {
      preparedFor = key
      load(1, '')
    }
    if (!dialog.open) dialog.showModal()
  }

  $: if (!open && dialog) {
    preparedFor = null
    if (dialog.open) dialog.close()
  }

  async function load(targetPage = page, targetResponse = response) {
    if (!policy) return
    loading = true
    error = ''
    try {
      const data = await fetchPolicyMachines(policy.uuid, {
        response: targetResponse,
        page: targetPage,
        countPerPage
      })
      machines = data.machines || []
      page = targetPage
      pageCount = data.page_count || 1
      totalCount = data.total_count || machines.length
      response = targetResponse
    } catch (err) {
      error = (err as Error).message || 'Failed to fetch policy machines'
    } finally {
      loading = false
    }
  }

  function selectTab(value: string) {
    if (value !== response) load(1, value)
  }

  function changePage(target: number) {
    if (target > 0 && target <= pageCount) load(target, response)
  }

  function responseVariant(value?: string): 'success' | 'danger' | 'warning' {
    if (value === 'passing') return 'success'
    if (value === 'failing') return 'danger'
    return 'warning'
  }

  function handleClose() {
    if (open) {
      open = false
      onClose()
    }
  }
</script>

<dialog bind:this={dialog} class="policy-machines" closedby="any" onclose={handleClose}>
  <header>
    <h2>{policy?.name || policy?.title || 'Policy'} hosts</h2>
    <p>{totalCount} {totalCount === 1 ? 'host' : 'hosts'}{response ? ` (${response})` : ''}</p>
  </header>

  <section class="vstack gap-3">
    <ErrorMessage message={error} onClose={() => (error = '')} />

    <div role="tablist">
      {#each tabs as tab}
        <button
          type="button"
          role="tab"
          aria-selected={response === tab.value}
          onclick={() => selectTab(tab.value)}
        >
          {tab.label}
        </button>
      {/each}
    </div>

    <div class="table" aria-busy={loading ? 'true' : undefined}>
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
          {#if loading}
            <tr><td colspan="6" class="align-center text-light">Loading hosts...</td></tr>
          {:else}
            {#each machines as machine}
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
              <tr><td colspan="6" class="align-center text-light">No hosts for this filter</td></tr>
            {/each}
          {/if}
        </tbody>
      </table>
    </div>
  </section>

  <footer class="hstack justify-between">
    <Pagination
      currentPage={page}
      {pageCount}
      disabled={loading}
      label="Policy hosts pagination"
      onPageChange={changePage}
    />
    <button type="button" class="outline" onclick={() => dialog.close()}>Close</button>
  </footer>
</dialog>

<style>
  .policy-machines {
    width: min(64rem, 92vw);
  }
  .policy-machines > section {
    max-height: 60vh;
  }
</style>
