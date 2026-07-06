<script lang="ts">
  import { onDestroy, onMount, untrack } from 'svelte'
  import { page } from '$app/state'
  import { goto, pushState } from '$app/navigation'
  import {
    deleteMachine,
    deleteMachineQuery,
    executeMachineQuery,
    fetchGroups,
    fetchMachine,
    fetchMachineMetrics,
    fetchMachinePolicies,
    fetchMachineQueries,
    fetchMachineQueryResults,
    fetchOwners,
    fetchMetricSchemas,
    updateMachine,
    updateMachineInventory,
    updateMachineGroups,
    machineQueryExportUrl,
    type AdHocQueryResults,
    type DeviceOwner,
    type Group,
    type Machine,
    type MachinePolicyPosture,
    type MachineQueryRecord,
    type Me,
    type MetricSchemas,
    type NodeMetrics
  } from '$lib/api'
  import { formatTimestamp, isOnline, machineHostname, machineOS } from '$lib/util'
  import { severityRank, severityVariant } from '$lib/severity'
  import { sortRows, type SortAccessors, type SortState } from '$lib/tableSort'
  import SortableHeader from '$lib/components/SortableHeader.svelte'
  import { rootShape, type JSONSchema } from '$lib/metricSchema'
  import MetricRenderer from '$lib/components/MetricRenderer.svelte'
  import MultiSelectDropdown from '$lib/components/MultiSelectDropdown.svelte'
  import SelectDropdown from '$lib/components/SelectDropdown.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import BadgeList from '$lib/components/BadgeList.svelte'
  import SqlEditor from '$lib/components/SqlEditor.svelte'
  import QueryResultTable from '$lib/components/QueryResultTable.svelte'
  import DownloadResultsButton from '$lib/components/DownloadResultsButton.svelte'
  import ActionsMenu from '$lib/components/ActionsMenu.svelte'
  import Truncate from '$lib/components/Truncate.svelte'
  import { canFrom, me, ownerOnlyFrom } from '$lib/auth'
  import Pencil from '@lucide/svelte/icons/pencil'
  import Save from '@lucide/svelte/icons/save'
  import Play from '@lucide/svelte/icons/play'
  import Trash2 from '@lucide/svelte/icons/trash-2'

  type OatTabsElement = HTMLElement & { activeIndex: number }

  let machine = $state<Machine | null>(null)
  let metrics = $state<NodeMetrics>({})
  let metricSchemas = $state<MetricSchemas>({ schemas: {}, kinds: [] })
  let queryText = $state('')
  let queryHistory = $state<MachineQueryRecord[]>([])
  let selectedQuery = $state<MachineQueryRecord | null>(null)
  let resultsOpen = $state(false)
  let resultsDialog = $state<HTMLDialogElement>()
  let queryResults = $state<AdHocQueryResults | null>(null)
  let queryResultsLoading = $state(false)
  let policyPosture = $state<MachinePolicyPosture[]>([])
  let loading = $state(true)
  let historyLoading = $state(false)
  let executing = $state(false)
  let error = $state('')
  let currentQueryPage = $state(1)
  let queryPageCount = $state(1)
  let queryTotalCount = $state(0)
  const queryCountPerPage = 10
  const queryPollIntervalMs = 3000
  const maxQueryPollAttempts = 20
  let pollTimer: ReturnType<typeof setTimeout> | null = null
  let pollAttempts = $state(0)
  let pollEpoch = $state(0)
  let deleteDialogOpen = $state(false)
  let queryToDelete = $state<MachineQueryRecord | null>(null)
  let deletingQuery = $state(false)
  let deleteMachineDialogOpen = $state(false)
  let deletingMachine = $state(false)
  let availableGroups = $state<Group[]>([])
  let selectedGroupIds = $state<string[]>([])
  let availableOwners = $state<DeviceOwner[]>([])
  let selectedOwnerId = $state('')
  let displayName = $state('')
  let internalTrackingId = $state('')
  let inventoryNotes = $state('')
  let editing = $state(false)
  let saving = $state(false)
  let tabs = $state<OatTabsElement>()
  const currentMe = $derived(($me ?? (page.data?.me as Me | null)) ?? null)
  const ownerOnlyMode = $derived(ownerOnlyFrom(currentMe))
  const tabSlugs = $derived(ownerOnlyMode ? ['overview', 'policies'] : ['overview', 'policies', 'query'])
  const activeTabIndex = $derived.by(() => {
    const i = tabSlugs.indexOf(page.url.hash.replace(/^#/, ''))
    return i >= 0 ? i : 0
  })
  let mounted = $state(false)
  let loadedMachineId = $state('')

  const machineId = $derived(page.params.id as string)
  $effect(() => {
    if (!tabs || tabs.activeIndex === activeTabIndex) return
    tabs.activeIndex = activeTabIndex
  })
  $effect(() => {
    if (!ownerOnlyMode && activeTabIndex === 2) untrack(ensurePolling)
  })
  // onMount only fires once; when the router reuses this component for a
  // different :id, reload and reset poll/edit state so we don't show stale
  // data or leak a poll timer onto the new machine.
  $effect(() => {
    if (mounted && machineId !== loadedMachineId) untrack(() => switchMachine(machineId))
  })
  const queryStartResult = $derived(queryTotalCount === 0 ? 0 : (currentQueryPage - 1) * queryCountPerPage + 1)
  const queryEndResult = $derived(Math.min(currentQueryPage * queryCountPerPage, queryTotalCount))
  const canViewQueryResult = $derived(canFrom(currentMe, 'query_result', 'view'))
  const queryResultsDownloadHref = $derived(
    canViewQueryResult &&
      selectedQuery?.id &&
      queryResults?.export_supported &&
      !queryResults.pending &&
      !queryResults.error &&
      (queryResults.total ?? 0) > 0
      ? machineQueryExportUrl(machineId, String(selectedQuery.id))
      : ''
  )

  onMount(() => {
    loadedMachineId = machineId
    mounted = true
    loadMachine()
  })
  onDestroy(stopPolling)

  function switchMachine(id: string) {
    loadedMachineId = id
    stopPolling()
    editing = false
    currentQueryPage = 1
    loadMachine()
  }

  async function loadMachine() {
    loading = true
    error = ''
    try {
      if (ownerOnlyMode) {
        const [machineData, policyData, metricsData, schemasData] = await Promise.all([
          fetchMachine(machineId),
          fetchMachinePolicies(machineId),
          fetchMachineMetrics(machineId).catch(() => ({ metrics: {} as NodeMetrics })),
          fetchMetricSchemas().catch(() => ({ schemas: {}, kinds: [] }) as MetricSchemas)
        ])
        machine = machineData
        selectedGroupIds = (machineData.groups || []).map((g) => g.uuid)
        selectedOwnerId = machineData.inventory?.owner?.uuid || ''
        displayName = machineData.display_name || ''
        internalTrackingId = machineData.inventory?.internal_tracking_id || ''
        inventoryNotes = machineData.inventory?.notes || ''
        queryHistory = []
        queryTotalCount = 0
        queryPageCount = 1
        availableGroups = []
        availableOwners = []
        policyPosture = Array.isArray(policyData)
          ? policyData
          : (policyData as { policies?: MachinePolicyPosture[] }).policies || []
        metrics = metricsData.metrics || {}
        metricSchemas = schemasData
        return
      }
      const [machineData, historyData, policyData, groupData, ownerData, metricsData, schemasData] = await Promise.all([
        fetchMachine(machineId),
        fetchMachineQueries(machineId, { page: currentQueryPage, countPerPage: queryCountPerPage }),
        fetchMachinePolicies(machineId),
        fetchGroups({ page: 1, countPerPage: 1000 }),
        fetchOwners({ page: 1, countPerPage: 1000 }),
        fetchMachineMetrics(machineId).catch(() => ({ metrics: {} as NodeMetrics })),
        fetchMetricSchemas().catch(() => ({ schemas: {}, kinds: [] }) as MetricSchemas)
      ])
      machine = machineData
      selectedGroupIds = (machineData.groups || []).map((g) => g.uuid)
      selectedOwnerId = machineData.inventory?.owner?.uuid || ''
      displayName = machineData.display_name || ''
      internalTrackingId = machineData.inventory?.internal_tracking_id || ''
      inventoryNotes = machineData.inventory?.notes || ''
      setQueryHistory(historyData)
      policyPosture = Array.isArray(policyData)
        ? policyData
        : (policyData as { policies?: MachinePolicyPosture[] }).policies || []
      availableGroups = groupData.groups || []
      availableOwners = ownerData.owners || []
      metrics = metricsData.metrics || {}
      metricSchemas = schemasData
    } catch (err) {
      error = (err as Error).message || 'Failed to load machine data'
    } finally {
      loading = false
    }
  }

  function setQueryHistory(data: any) {
    queryHistory = Array.isArray(data) ? data : data.queries || []
    queryTotalCount = Array.isArray(data) ? queryHistory.length : data.total_count || 0
    queryPageCount = Math.max(1, Array.isArray(data) ? 1 : data.page_count || 1)
  }

  async function loadQueryHistory(targetPage = currentQueryPage) {
    historyLoading = true
    error = ''
    try {
      const data = await fetchMachineQueries(machineId, { page: targetPage, countPerPage: queryCountPerPage })
      currentQueryPage = targetPage
      setQueryHistory(data)
    } catch (err) {
      error = (err as Error).message || 'Failed to load query history'
    } finally {
      historyLoading = false
    }
  }

  // Bumping the epoch invalidates any in-flight silent poll so it neither
  // reschedules nor clobbers the view after the user has navigated.
  function stopPolling() {
    pollEpoch += 1
    if (pollTimer) {
      clearTimeout(pollTimer)
      pollTimer = null
    }
  }

  function hasPendingQueries() {
    return queryHistory.some((query) => query.status === 'pending')
  }

  // osquery returns ad-hoc query results asynchronously on its distributed_interval
  // (~10s), so poll quietly until the newest query leaves the 'pending' state.
  function startPollingForResults() {
    stopPolling()
    pollAttempts = 0
    pollTimer = setTimeout(pollForResults, queryPollIntervalMs)
  }

  // Resume polling when returning to the Query tab (or after a remount) while
  // results are still pending and no poll loop is already running.
  function ensurePolling() {
    if (pollTimer || currentQueryPage !== 1) return
    if (hasPendingQueries()) startPollingForResults()
  }

  // Quiet refresh that fetches page 1 directly (no spinner, no row collapse) and
  // applies results only if the user hasn't navigated away mid-request.
  async function pollForResults() {
    pollTimer = null
    if (currentQueryPage !== 1) return
    const epoch = pollEpoch
    pollAttempts += 1
    try {
      const data = await fetchMachineQueries(machineId, { page: 1, countPerPage: queryCountPerPage })
      if (epoch !== pollEpoch || currentQueryPage !== 1) return
      setQueryHistory(data)
    } catch {
      // transient failure — keep existing results and retry below
    }
    if (epoch === pollEpoch && currentQueryPage === 1 && hasPendingQueries() && pollAttempts < maxQueryPollAttempts) {
      pollTimer = setTimeout(pollForResults, queryPollIntervalMs)
    }
  }

  async function runQuery() {
    if (!canExecuteMachine || !queryText.trim()) return
    executing = true
    error = ''
    try {
      await executeMachineQuery(machineId, queryText)
      queryText = ''
      await loadQueryHistory(1)
      startPollingForResults()
    } catch (err) {
      error = (err as Error).message || 'Query execution failed'
    } finally {
      executing = false
    }
  }

  async function changeQueryPage(targetPage: number) {
    if (targetPage < 1 || targetPage > queryPageCount || targetPage === currentQueryPage || historyLoading) return
    stopPolling()
    await loadQueryHistory(targetPage)
    // Landing back on page 1 with results still pending should resume polling.
    ensurePolling()
  }

  function confirmDeleteQuery(query: MachineQueryRecord) {
    if (!canDeleteQueryResult || !query?.id) return
    queryToDelete = query
    deleteDialogOpen = true
  }

  async function deleteSelectedQuery() {
    if (!queryToDelete?.id) return
    deletingQuery = true
    error = ''
    try {
      await deleteMachineQuery(machineId, queryToDelete.id)
      const targetPage = queryHistory.length === 1 && currentQueryPage > 1 ? currentQueryPage - 1 : currentQueryPage
      deleteDialogOpen = false
      queryToDelete = null
      await loadQueryHistory(targetPage)
    } catch (err) {
      error = (err as Error).message || 'Failed to delete query'
    } finally {
      deletingQuery = false
    }
  }

  async function deleteSelectedMachine() {
    if (!canDeleteMachine) return
    deletingMachine = true
    error = ''
    try {
      await deleteMachine(machineId)
      deleteMachineDialogOpen = false
      await goto('/inventory')
    } catch (err) {
      error = (err as Error).message || 'Failed to delete host'
    } finally {
      deletingMachine = false
    }
  }

  function setTab(index: number) {
    const slug = tabSlugs[index]
    // Skip if already on this tab: ot-tabs echoes activations back through
    // ot-tab-change, so guarding here keeps each switch to one history entry.
    if (page.url.hash.replace(/^#/, '') === slug) return
    const url = new URL(page.url)
    url.hash = slug
    pushState(url, {})
  }

  function handleTabChange(event: CustomEvent<{ index: number }>) {
    setTab(event.detail.index)
  }

  function seedEditFields() {
    selectedGroupIds = (machine?.groups || []).map((g) => g.uuid)
    selectedOwnerId = machine?.inventory?.owner?.uuid || ''
    displayName = machine?.display_name || ''
    internalTrackingId = machine?.inventory?.internal_tracking_id || ''
    inventoryNotes = machine?.inventory?.notes || ''
  }

  function startEdit() {
    if (!canEditOverview) return
    seedEditFields()
    setTab(0)
    editing = true
  }

  function cancelEdit() {
    seedEditFields()
    editing = false
  }

  async function saveOverview() {
    if (!canEditOverview) return
    saving = true
    error = ''
    try {
      // Settle both so a partial failure can be reported precisely instead of a
      // generic message that hides which write already landed. Both endpoints are
      // idempotent, so staying in edit mode lets the user simply retry.
      const [machineResult, inventoryResult, groupsResult] = await Promise.allSettled([
        canUpdateMachine ? updateMachine(machineId, { display_name: displayName }) : Promise.resolve(),
        canUpdateInventory
          ? updateMachineInventory(machineId, {
              owner_id: selectedOwnerId || null,
              internal_tracking_id: internalTrackingId,
              notes: inventoryNotes
            })
          : Promise.resolve(),
        canUpdateMachine ? updateMachineGroups(machineId, selectedGroupIds) : Promise.resolve()
      ])
      const failed: string[] = []
      if (machineResult.status === 'rejected') failed.push('display name')
      if (inventoryResult.status === 'rejected') failed.push('inventory')
      if (groupsResult.status === 'rejected') failed.push('groups')
      if (failed.length > 0) {
        const reason =
          machineResult.status === 'rejected'
            ? machineResult.reason
            : inventoryResult.status === 'rejected'
            ? inventoryResult.reason
            : groupsResult.status === 'rejected'
              ? groupsResult.reason
              : undefined
        error = `Failed to update ${failed.join(' and ')}: ${(reason as Error)?.message || 'unknown error'}`
        return
      }
      await loadMachine()
      editing = false
    } catch (err) {
      error = (err as Error).message || 'Failed to update machine'
    } finally {
      saving = false
    }
  }

  function postureVariant(response?: string): 'success' | 'danger' | 'warning' {
    if (response === 'passing') return 'success'
    if (response === 'failing') return 'danger'
    return 'warning'
  }

  // Sortable policy tab. Response is ranked failing → unknown → passing so the
  // most actionable rows surface first; severity uses the canonical rank.
  const responseRank: Record<string, number> = { failing: 0, unknown: 1, passing: 2 }

  type PostureSortCol = 'severity' | 'response'
  const postureAccessors: SortAccessors<MachinePolicyPosture, PostureSortCol> = {
    severity: (p) => severityRank[p.severity || 'medium'] ?? 99,
    response: (p) => responseRank[p.response || 'unknown'] ?? 99
  }
  let postureSort = $state<SortState<PostureSortCol>>(null)
  const sortedPosture = $derived(
    sortRows(policyPosture, postureSort, postureAccessors, (p) => p.name || p.title)
  )

  function statusVariant(query: MachineQueryRecord): 'success' | 'danger' | 'warning' {
    if (query.status === 'complete') return 'success'
    if (query.status === 'error') return 'danger'
    return 'warning'
  }

  function openQueryResults(query: MachineQueryRecord) {
    selectedQuery = query
    resultsOpen = true
    loadQueryResults(1)
  }

  function handleResultsClose() {
    resultsOpen = false
    selectedQuery = null
    queryResults = null
  }

  async function loadQueryResults(targetPage: number) {
    if (!selectedQuery?.id) return
    queryResultsLoading = true
    try {
      queryResults = await fetchMachineQueryResults(machineId, String(selectedQuery.id), { page: targetPage })
    } catch (err) {
      queryResults = {
        columns: [],
        rows: [],
        total: 0,
        page: 1,
        count_per_page: 0,
        page_count: 0,
        error: (err as Error).message || 'Failed to load results'
      }
    } finally {
      queryResultsLoading = false
    }
  }

  $effect(() => {
    if (!resultsDialog) return
    if (resultsOpen && !resultsDialog.open) resultsDialog.showModal()
    else if (!resultsOpen && resultsDialog.open) resultsDialog.close()
  })

  const visibleMetricKinds = $derived((metricSchemas.kinds || []).filter((k) => metrics[k]))
  const summaryMetricKinds = $derived(visibleMetricKinds.filter(
    (k) => rootShape(metricSchemas.schemas[k] as JSONSchema).kind === 'card'
  ))
  const tableMetricKinds = $derived(visibleMetricKinds.filter(
    (k) => rootShape(metricSchemas.schemas[k] as JSONSchema).kind === 'table'
  ))
  const canUpdateMachine = $derived(canFrom($me, 'machine', 'update'))
  const canUpdateInventory = $derived(canFrom($me, 'inventory', 'update'))
  const canEditOverview = $derived(canUpdateMachine || canUpdateInventory)
  const canExecuteMachine = $derived(canFrom($me, 'machine', 'execute'))
  const canDeleteQueryResult = $derived(canFrom($me, 'query_result', 'delete'))
  const canDeleteMachine = $derived(canFrom($me, 'machine', 'delete'))
</script>

<section class="vstack gap-4">
  {#if loading}
    <Spinner fill />
  {:else}
    <header class="hstack justify-between mb-4">
      <div>
        <h1 class="mb-2">{machineHostname(machine!)}</h1>
        <p class="text-light">
          {machineOS(machine!)}
          {machine?.inventory?.internal_tracking_id ? ` · ${machine.inventory.internal_tracking_id}` : ''}
        </p>
      </div>
      <div class="hstack gap-2">
        <span class="badge" data-variant={isOnline(machine) ? 'success' : 'danger'}>
          {isOnline(machine) ? 'Online' : 'Offline'}
        </span>
        {#if editing}
          <button type="button" class="outline" onclick={cancelEdit} disabled={saving}>Cancel</button>
          <button type="button" class="gap-1" onclick={saveOverview} disabled={saving} aria-busy={saving ? 'true' : undefined} data-spinner="small">
            <Save size={16} aria-hidden="true" />
            {saving ? 'Saving...' : 'Save'}
          </button>
        {:else}
          {#if canEditOverview}
            <button type="button" class="gap-1" onclick={startEdit}>
              <Pencil size={16} aria-hidden="true" />
              Edit
            </button>
          {/if}
          {#if canDeleteMachine}
            <button type="button" class="outline gap-1" data-variant="danger" onclick={() => (deleteMachineDialogOpen = true)}>
              <Trash2 size={16} aria-hidden="true" />
              Delete host
            </button>
          {/if}
        {/if}
      </div>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <ot-tabs bind:this={tabs} onot-tab-change={handleTabChange}>
      <div role="tablist" aria-label="Machine sections">
        <button type="button" role="tab" aria-selected={activeTabIndex === 0} onclick={() => setTab(0)}>
          Overview
        </button>
        <button type="button" role="tab" aria-selected={activeTabIndex === 1} onclick={() => setTab(1)}>
          Policies
        </button>
        {#if !ownerOnlyMode}
          <button type="button" role="tab" aria-selected={activeTabIndex === 2} onclick={() => setTab(2)}>
            Query
          </button>
        {/if}
      </div>

      <div role="tabpanel">
        <div class="vstack gap-4">
          {#if editing}
            <div class="vstack gap-3">
              {#if canUpdateMachine}
                <label data-field>
                  Display name
                  <input bind:value={displayName} maxlength="255" placeholder={machine?.hostname || ''} />
                </label>
              {/if}
              {#if canUpdateInventory}
                <div class="row">
                  <div class="col-6">
                    <SelectDropdown
                      label="Owner"
                      options={[
                        { value: '', label: 'Unassigned' },
                        ...availableOwners.map((owner) => ({
                          value: owner.uuid,
                          label: owner.display_name || owner.email || owner.uuid
                        }))
                      ]}
                      bind:value={selectedOwnerId}
                    />
                  </div>
                  <div class="col-6">
                    <label data-field>
                      Internal tracking ID
                      <input bind:value={internalTrackingId} placeholder="ASSET-10042" />
                    </label>
                  </div>
                </div>
              {/if}
              {#if canUpdateMachine}
                <MultiSelectDropdown
                  label="Assigned Groups"
                  options={availableGroups}
                  bind:value={selectedGroupIds}
                  placeholder="No groups assigned"
                  searchPlaceholder="Search groups..."
                  emptyLabel="No groups available yet"
                />
              {/if}
              {#if canUpdateInventory}
                <label data-field>
                  Notes
                  <textarea bind:value={inventoryNotes} rows="3"></textarea>
                </label>
              {/if}
            </div>
          {:else}
            <dl class="facts">
              <div>
                <dt>Hostname</dt>
                <dd>{machine?.hostname || '—'}</dd>
              </div>
              {#if machine?.display_name && machine.display_name !== machine.hostname}
                <div>
                  <dt>Display name</dt>
                  <dd>{machine.display_name}</dd>
                </div>
              {/if}
              <div>
                <dt>Owner</dt>
                <dd>{machine?.inventory?.owner?.display_name || machine?.inventory?.owner?.email || 'Unassigned'}</dd>
              </div>
              <div>
                <dt>Internal tracking ID</dt>
                <dd>{machine?.inventory?.internal_tracking_id || '—'}</dd>
              </div>
              <div>
                <dt>Groups</dt>
                <dd><BadgeList items={machine?.groups || []} max={99} /></dd>
              </div>
              <div class="full">
                <dt>Notes</dt>
                <dd class="notes">{machine?.inventory?.notes || '—'}</dd>
              </div>
            </dl>
          {/if}

          <section class="vstack gap-3">
            <h3>Device Metrics</h3>
            {#if visibleMetricKinds.length > 0}
              {#if summaryMetricKinds.length > 0}
                <div class="metric-grid">
                  {#each summaryMetricKinds as kind}
                    <MetricRenderer
                      {kind}
                      schema={metricSchemas.schemas[kind] as JSONSchema}
                      entry={metrics[kind]}
                    />
                  {/each}
                </div>
              {/if}
              {#each tableMetricKinds as kind}
                <MetricRenderer
                  {kind}
                  schema={metricSchemas.schemas[kind] as JSONSchema}
                  entry={metrics[kind]}
                />
              {/each}
            {:else}
              <p class="text-light">No metrics reported for this machine</p>
            {/if}
          </section>
        </div>
      </div>

      <div role="tabpanel">
        <div class="table">
          <table>
            <thead>
              <tr>
                <th>Policy</th>
                <SortableHeader bind:state={postureSort} col="severity" label="Severity" />
                <SortableHeader bind:state={postureSort} col="response" label="Response" />
                <th>Checked</th>
                <th>Error</th>
                <th>Resolution</th>
              </tr>
            </thead>
            <tbody>
              {#each sortedPosture as policy}
                <tr>
                  <td>
                    <strong>{policy.name || policy.title}</strong>
                    {#if policy.description}<p class="text-light">{policy.description}</p>{/if}
                  </td>
                  <td>
                    <span class="badge" data-variant={severityVariant[policy.severity || 'medium']}>
                      {policy.severity || 'medium'}
                    </span>
                  </td>
                  <td>
                    <span class="badge posture-badge" data-variant={postureVariant(policy.response)}>
                      {policy.stale ? `${policy.response} stale` : policy.response}
                    </span>
                  </td>
                  <td>{formatTimestamp(policy.checked_at)}</td>
                  <td>{policy.last_error || ''}</td>
                  <td>{policy.response === 'failing' ? policy.resolution || '' : ''}</td>
                </tr>
              {:else}
                <tr>
                  <td colspan="6" class="align-center text-light">No policies target this machine</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>

      {#if !ownerOnlyMode}
        <div role="tabpanel">
          <div class="vstack gap-4">
            {#if canExecuteMachine}
              <form onsubmit={(e) => { e.preventDefault(); runQuery() }}>
                <label data-field>
                  SQL Query
                  <SqlEditor
                    bind:value={queryText}
                    minLines={6}
                    lineNumbers
                    placeholder="SELECT * FROM processes LIMIT 10;"
                    ariaLabel="Ad-hoc SQL query"
                    onsubmit={() => { if (!executing && queryText.trim()) runQuery() }}
                  />
                </label>
                <footer class="hstack justify-end mt-4">
                  <button
                    type="submit"
                    class="gap-1"
                    disabled={executing || !queryText.trim()}
                    aria-busy={executing ? 'true' : undefined}
                    data-spinner="small"
                  >
                    <Play size={16} aria-hidden="true" />
                    {executing ? 'Executing...' : 'Run Query'}
                  </button>
                </footer>
              </form>
            {/if}

            <div class="hstack justify-between">
              <h2>Query History</h2>
              <p class="text-light">
                Showing <strong>{queryStartResult}</strong> to <strong>{queryEndResult}</strong> of
                <strong>{queryTotalCount}</strong> results
              </p>
            </div>
            {#if historyLoading}
              <Spinner />
            {:else}
              <div class="table">
                <table class="queries-table">
                  <thead>
                    <tr>
                      <th class="col-id">ID</th>
                      <th class="col-query">Query</th>
                      <th>Status</th>
                      <th>Rows</th>
                      <th>Executed</th>
                      <th class="col-actions"><span class="sr-only">Actions</span></th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each queryHistory as query}
                      <tr>
                        <td>
                          <button type="button" class="cell-link" onclick={() => openQueryResults(query)}>
                            <Truncate text={String(query.id ?? '')} />
                          </button>
                        </td>
                        <td class="text-light"><code class="query-cell"><Truncate text={query.query || ''} /></code></td>
                        <td>
                          {#if query.status}
                            <span class="badge" data-variant={statusVariant(query)}>{query.status}</span>
                          {/if}
                        </td>
                        <td>{query.row_count ?? 0}</td>
                        <td class="text-light">{formatTimestamp(query.timestamp)}</td>
                        <td class="col-actions">
                          {#if canDeleteQueryResult}
                            <ActionsMenu label="Actions for query result">
                              <button role="menuitem" type="button" onclick={() => confirmDeleteQuery(query)}>Delete</button>
                            </ActionsMenu>
                          {/if}
                        </td>
                      </tr>
                    {:else}
                      <tr><td colspan="6" class="align-center text-light">No queries executed yet</td></tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {/if}
            {#if queryTotalCount > 0}
              <footer class="hstack justify-end">
                <Pagination
                  currentPage={currentQueryPage}
                  pageCount={queryPageCount}
                  disabled={historyLoading}
                  label="Query history pagination"
                  onPageChange={changeQueryPage}
                />
              </footer>
            {/if}
          </div>
        </div>
      {/if}
    </ot-tabs>
  {/if}
</section>

<ConfirmDialog
  bind:open={deleteDialogOpen}
  title="Delete Query Result"
  message="Are you sure you want to delete this query result? This action cannot be undone."
  confirming={deletingQuery}
  confirmingLabel="Deleting..."
  onConfirm={deleteSelectedQuery}
  onCancel={() => (queryToDelete = null)}
/>

<ConfirmDialog
  bind:open={deleteMachineDialogOpen}
  title="Delete"
  message="Deleting this host removes it and all of its collected data. Its agent will be forced to re-enroll on its next check-in. This action cannot be undone."
  confirmLabel="Delete host"
  confirming={deletingMachine}
  confirmingLabel="Deleting..."
  onConfirm={deleteSelectedMachine}
/>

<dialog bind:this={resultsDialog} class="query-results-modal" closedby="any" onclose={handleResultsClose}>
  {#if selectedQuery}
    <header>
      <h2>Query Result</h2>
      <p class="hstack gap-2">
        {#if selectedQuery.status}
          <span class="badge" data-variant={statusVariant(selectedQuery)}>{selectedQuery.status}</span>
        {/if}
        <span class="text-light">{selectedQuery.row_count ?? 0} row{selectedQuery.row_count === 1 ? '' : 's'}</span>
        {#if queryResultsDownloadHref}
          <DownloadResultsButton href={queryResultsDownloadHref} />
        {/if}
      </p>
    </header>
    <section>
      <code class="query-text">{selectedQuery.query}</code>
      <QueryResultTable
        columns={queryResults?.columns ?? []}
        rows={queryResults?.rows ?? []}
        total={queryResults?.total ?? 0}
        page={queryResults?.page ?? 1}
        pageCount={queryResults?.page_count ?? 1}
        loading={queryResultsLoading}
        pending={queryResults?.pending ?? false}
        error={queryResults?.error ?? selectedQuery.error ?? ''}
        browsingDisabled={queryResults?.browsing_disabled ?? false}
        onPageChange={(p) => loadQueryResults(p)}
      />
    </section>
  {/if}
  <footer class="hstack justify-end">
    <button type="button" class="outline" onclick={() => resultsDialog?.close()}>Close</button>
  </footer>
</dialog>

<style>
  .posture-badge {
    white-space: nowrap;
  }
  .query-text {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .queries-table {
    table-layout: fixed;
    width: 100%;
  }
  .queries-table .col-id {
    width: 22%;
  }
  .queries-table .col-query {
    width: 44%;
  }
  .queries-table .col-actions {
    width: 3rem;
    text-align: right;
  }
  /* .cell-link is display:inline globally, so it can't clip; make the id link
     block-level so the UUID truncates to the column instead of overflowing. */
  .queries-table .cell-link {
    display: block;
    min-width: 0;
    max-width: 100%;
  }
  .query-cell {
    display: block;
    min-width: 0;
  }
  .query-results-modal {
    width: min(72rem, 94vw);
  }
  .query-results-modal > section {
    max-height: 64vh;
    overflow: auto;
  }
  .metric-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(16rem, 1fr));
    gap: var(--space-3, 1rem);
    align-items: stretch;
  }
  .facts {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: var(--space-4, 1.25rem) var(--space-8, 2rem);
    margin: 0;
  }
  .facts > div {
    min-width: 0;
  }
  .facts > .full {
    grid-column: 1 / -1;
  }
  .facts dt {
    margin: 0 0 var(--space-1, 0.25rem);
    font-size: var(--text-8);
    color: var(--muted-foreground);
  }
  .facts dd {
    margin: 0;
    overflow-wrap: anywhere;
  }
  .facts dd.notes {
    white-space: pre-wrap;
  }
</style>
