<script lang="ts">
  import { onMount } from 'svelte'
  import { goto } from '$app/navigation'
  import {
    createQueryRun,
    deleteQueryRun as apiDeleteQueryRun,
    fetchGroups,
    fetchMachines,
    fetchQueryRuns,
    previewQueryTargets,
    type Group,
    type Machine,
    type QueryRun
  } from '$lib/api'
  import { formatTimestamp, machineHostname } from '$lib/util'
  import { canFrom, me } from '$lib/auth'
  import SqlEditor from '$lib/components/SqlEditor.svelte'
  import MultiSelectDropdown from '$lib/components/MultiSelectDropdown.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import ActionsMenu from '$lib/components/ActionsMenu.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import Truncate from '$lib/components/Truncate.svelte'
  import Play from '@lucide/svelte/icons/play'

  const platformOptions = [
    { value: 'linux', label: 'Linux' },
    { value: 'darwin', label: 'macOS' },
    { value: 'windows', label: 'Windows' },
    { value: 'posix', label: 'POSIX' }
  ]

  const runCountPerPage = 10

  let queryText = $state('')
  let availableMachines = $state<Machine[]>([])
  let availableGroups = $state<Group[]>([])
  let selectedHostIds = $state<string[]>([])
  let selectedGroupIds = $state<string[]>([])
  let selectedPlatforms = $state<string[]>([])

  let runs = $state<QueryRun[]>([])
  let currentPage = $state(1)
  let pageCount = $state(1)
  let totalCount = $state(0)
  let loading = $state(true)
  let executing = $state(false)
  let error = $state('')

  let previewCount = $state<number | null>(null)
  let previewing = $state(false)
  let previewTimer: ReturnType<typeof setTimeout> | undefined

  let deleteOpen = $state(false)
  let runToDelete = $state<QueryRun | null>(null)
  let isDeleting = $state(false)

  const canExecute = $derived(canFrom($me, 'machine', 'execute'))
  const canViewRuns = $derived(canFrom($me, 'query_result', 'view'))
  const canDeleteRuns = $derived(canFrom($me, 'query_result', 'delete'))

  const hostOptions = $derived(
    availableMachines.map((m) => ({ value: m.uuid, label: machineHostname(m) }))
  )
  const hasTargets = $derived(
    selectedHostIds.length > 0 || selectedGroupIds.length > 0 || selectedPlatforms.length > 0
  )
  const startResult = $derived(runs.length === 0 ? 0 : (currentPage - 1) * runCountPerPage + 1)
  const endResult = $derived(Math.min(currentPage * runCountPerPage, totalCount))

  onMount(load)

  // Debounced target preview so the "N hosts targeted" hint reflects the
  // current host/group/platform selection without a request per keystroke.
  $effect(() => {
    const payload = {
      host_ids: [...selectedHostIds],
      group_ids: [...selectedGroupIds],
      platforms: [...selectedPlatforms]
    }
    clearTimeout(previewTimer)
    if (!hasTargets) {
      previewCount = 0
      return
    }
    previewing = true
    previewTimer = setTimeout(async () => {
      try {
        const res = await previewQueryTargets(payload)
        previewCount = res.host_count
      } catch {
        previewCount = null
      } finally {
        previewing = false
      }
    }, 300)
  })

  async function load() {
    loading = true
    error = ''
    try {
      const [machineData, groupData, runData] = await Promise.all([
        fetchMachines({ page: 1, countPerPage: 1000 }),
        fetchGroups({ page: 1, countPerPage: 1000 }),
        fetchQueryRuns({ page: currentPage, countPerPage: runCountPerPage })
      ])
      availableMachines = machineData.machines || []
      availableGroups = groupData.groups || []
      setRuns(runData)
    } catch (err) {
      error = (err as Error).message || 'Failed to load query page'
    } finally {
      loading = false
    }
  }

  function setRuns(data: { runs?: QueryRun[]; total_count?: number; page_count?: number }) {
    runs = data.runs || []
    totalCount = data.total_count || runs.length
    pageCount = Math.max(1, data.page_count || 1)
  }

  async function loadRuns(targetPage = currentPage) {
    try {
      const data = await fetchQueryRuns({ page: targetPage, countPerPage: runCountPerPage })
      currentPage = targetPage
      setRuns(data)
    } catch (err) {
      error = (err as Error).message || 'Failed to load query runs'
    }
  }

  function changePage(target: number) {
    if (target < 1 || target > pageCount || target === currentPage) return
    void loadRuns(target)
  }

  async function runQuery() {
    if (!canExecute || !queryText.trim() || !hasTargets) return
    executing = true
    error = ''
    try {
      const run = await createQueryRun({
        query: queryText,
        host_ids: selectedHostIds,
        group_ids: selectedGroupIds,
        platforms: selectedPlatforms
      })
      goto(`/query/${run.id}`)
    } catch (err) {
      error = (err as Error).message || 'Failed to run query'
      executing = false
    }
  }

  function confirmDelete(run: QueryRun) {
    if (!canDeleteRuns) return
    runToDelete = run
    deleteOpen = true
  }

  async function deleteRun() {
    if (!runToDelete) return
    isDeleting = true
    error = ''
    try {
      await apiDeleteQueryRun(runToDelete.id)
      deleteOpen = false
      runToDelete = null
      const targetPage = runs.length === 1 && currentPage > 1 ? currentPage - 1 : currentPage
      await loadRuns(targetPage)
    } catch (err) {
      error = (err as Error).message || 'Failed to delete query run'
    } finally {
      isDeleting = false
    }
  }
</script>

<section class="vstack gap-4">
  {#if loading}
    <Spinner fill />
  {:else}
    <header class="mb-4">
      <h1 class="mb-2">Query</h1>
      <p class="text-light">Run an osquery statement across many hosts at once</p>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    {#if canExecute}
      <article class="card vstack gap-4">
        <form onsubmit={(e) => { e.preventDefault(); runQuery() }} class="vstack gap-4">
          <label data-field>
            SQL Query
            <SqlEditor
              bind:value={queryText}
              minLines={6}
              lineNumbers
              placeholder="SELECT * FROM os_version;"
              ariaLabel="Multi-host SQL query"
              onsubmit={() => { if (!executing && queryText.trim() && hasTargets) runQuery() }}
            />
          </label>

          <div class="row">
            <div class="col-4">
              <MultiSelectDropdown
                label="Hosts"
                options={hostOptions}
                bind:value={selectedHostIds}
                placeholder="Select hosts"
                searchPlaceholder="Search hosts..."
                emptyLabel="No hosts available"
              />
            </div>
            <div class="col-4">
              <MultiSelectDropdown
                label="Groups"
                options={availableGroups}
                bind:value={selectedGroupIds}
                placeholder="Select groups"
                searchPlaceholder="Search groups..."
                emptyLabel="No groups available"
              />
            </div>
            <div class="col-4">
              <MultiSelectDropdown
                label="Platforms"
                options={platformOptions}
                bind:value={selectedPlatforms}
                placeholder="Select platforms"
                searchPlaceholder="Search platforms..."
                emptyLabel="No platforms available"
              />
            </div>
          </div>

          <footer class="hstack justify-between">
            <p class="text-light">
              {#if !hasTargets}
                No targets selected
              {:else if previewing}
                Resolving targets…
              {:else if previewCount === null}
                Could not resolve targets
              {:else}
                <strong>{previewCount}</strong> host{previewCount === 1 ? '' : 's'} targeted
              {/if}
            </p>
            <button
              type="submit"
              class="gap-1"
              disabled={executing || !queryText.trim() || !hasTargets}
              aria-busy={executing ? 'true' : undefined}
              data-spinner="small"
            >
              <Play size={16} aria-hidden="true" />
              {executing ? 'Running…' : 'Run Query'}
            </button>
          </footer>
        </form>
      </article>
    {/if}

    {#if canViewRuns}
      <div class="hstack justify-between">
        <h2>Recent Runs</h2>
        <p class="text-light">
          Showing <strong>{startResult}</strong> to <strong>{endResult}</strong> of <strong>{totalCount}</strong> runs
        </p>
      </div>

      <div class="table">
        <table class="runs-table">
          <thead>
            <tr>
              <th class="col-id">Run ID</th>
              <th class="col-query">Query</th>
              <th>Hosts</th>
              <th>Created</th>
              <th class="col-actions"><span class="sr-only">Actions</span></th>
            </tr>
          </thead>
          <tbody>
            {#each runs as run}
              <tr>
                <td>
                  <a href="/query/{run.id}" class="cell-link"><Truncate text={run.id} /></a>
                </td>
                <td class="text-light"><code class="query-cell"><Truncate text={run.query || ''} /></code></td>
                <td>{run.host_count ?? 0}</td>
                <td class="text-light">{formatTimestamp(run.created_at)}</td>
                <td class="col-actions">
                  {#if canDeleteRuns}
                    <ActionsMenu label={`Actions for query run`}>
                      <button role="menuitem" type="button" onclick={() => confirmDelete(run)}>Delete</button>
                    </ActionsMenu>
                  {/if}
                </td>
              </tr>
            {:else}
              <tr><td colspan="5" class="align-center text-light">No query runs yet</td></tr>
            {/each}
          </tbody>
        </table>
      </div>

      {#if totalCount > 0}
        <footer class="hstack justify-end">
          <Pagination {currentPage} {pageCount} label="Query runs pagination" onPageChange={changePage} />
        </footer>
      {/if}
    {/if}
  {/if}
</section>

<ConfirmDialog
  bind:open={deleteOpen}
  title="Delete Query Run"
  message="Are you sure you want to delete this query run and all its results? This action cannot be undone."
  confirming={isDeleting}
  confirmingLabel="Deleting..."
  onConfirm={deleteRun}
  onCancel={() => (runToDelete = null)}
/>

<style>
  .runs-table {
    table-layout: fixed;
    width: 100%;
  }
  .runs-table .col-id {
    width: 28%;
  }
  .runs-table .col-query {
    width: 44%;
  }
  .runs-table .col-actions {
    width: 3rem;
    text-align: right;
  }
  /* .cell-link is display:inline globally, so it can't clip; make the run-id
     link block-level so the UUID truncates to the column instead of overflowing. */
  .runs-table .cell-link {
    display: block;
    min-width: 0;
    max-width: 100%;
  }
  .query-cell {
    display: block;
    min-width: 0;
  }
</style>
