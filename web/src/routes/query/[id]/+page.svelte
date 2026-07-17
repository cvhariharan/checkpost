<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import { page } from '$app/state'
  import {
    fetchQueryRun,
    fetchQueryRunHostResults,
    queryRunExportUrl,
    queryRunHostExportUrl,
    type QueryRun,
    type QueryRunHost,
    type AdHocQueryResults
  } from '$lib/api'
  import { formatTimestamp } from '$lib/util'
  import { canFrom, me } from '$lib/auth'
  import { sortRows, type SortAccessors, type SortState } from '$lib/tableSort'
  import SortableHeader from '$lib/components/SortableHeader.svelte'
  import QueryResultTable from '$lib/components/QueryResultTable.svelte'
  import DownloadResultsButton from '$lib/components/DownloadResultsButton.svelte'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Spinner from '$lib/components/Spinner.svelte'

  const pollIntervalMs = 3000
  const maxPollAttempts = 40

  let run = $state<QueryRun | null>(null)
  let loading = $state(true)
  let error = $state('')

  let selectedHostId = $state('')
  let resultsOpen = $state(false)
  let resultsDialog = $state<HTMLDialogElement>()
  let hostResults = $state<AdHocQueryResults | null>(null)
  let hostLoading = $state(false)
  let runExportSupported = $state(false)
  let pollTimer: ReturnType<typeof setTimeout> | null = null
  let pollAttempts = 0
  let pollEpoch = 0

  type HostSortCol = 'host' | 'user' | 'platform' | 'status' | 'rows' | 'completed'
  const hostAccessors: SortAccessors<QueryRunHost, HostSortCol> = {
    host: (h) => h.hostname || h.node_uuid,
    user: (h) => h.owner_name || h.owner_email,
    platform: (h) => h.platform,
    status: (h) => h.status,
    rows: (h) => h.row_count ?? null,
    completed: (h) => h.timestamp
  }
  let hostSort = $state<SortState<HostSortCol>>(null)
  // Client-side sort of the loaded hosts; re-applies on each poll since it's derived.
  const sortedHosts = $derived(sortRows(run?.hosts ?? [], hostSort, hostAccessors, (h) => h.hostname || h.node_uuid))

  const runId = $derived(page.params.id as string)
  // Resolve the open host from the live run so polling updates flow into the modal.
  const selectedHost = $derived(run?.hosts?.find((h) => h.query_id === selectedHostId) || null)
  const canViewQueryResult = $derived(canFrom($me, 'query_result', 'view'))
  const completedHostsWithRows = $derived(run?.hosts?.filter((h) => h.status === 'complete' && (h.row_count ?? 0) > 0) ?? [])
  const runDownloadHref = $derived(canViewQueryResult && runExportSupported && completedHostsWithRows.length > 0 ? queryRunExportUrl(runId) : '')
  const hostDownloadHref = $derived(
    canViewQueryResult &&
      selectedHost?.query_id &&
      hostResults?.export_supported &&
      !hostResults.pending &&
      !hostResults.error &&
      (hostResults.total ?? 0) > 0
      ? queryRunHostExportUrl(runId, selectedHost.query_id)
      : ''
  )

  onMount(load)
  onDestroy(stopPolling)

  async function load() {
    loading = true
    error = ''
    try {
      run = await fetchQueryRun(runId)
      await loadRunExportSupport(run)
      if (hasPendingHosts(run)) startPolling()
    } catch (err) {
      error = (err as Error).message || 'Failed to load query run'
    } finally {
      loading = false
    }
  }

  function hasPendingHosts(value: QueryRun | null): boolean {
    return !!value?.hosts?.some((h) => h.status === 'pending')
  }

  // Bumping the epoch invalidates any in-flight poll so it neither reschedules
  // nor clobbers the view after the user navigates away.
  function stopPolling() {
    pollEpoch += 1
    if (pollTimer) {
      clearTimeout(pollTimer)
      pollTimer = null
    }
  }

  function startPolling() {
    stopPolling()
    pollAttempts = 0
    pollTimer = setTimeout(pollRun, pollIntervalMs)
  }

  async function pollRun() {
    pollTimer = null
    const epoch = pollEpoch
    pollAttempts += 1
    try {
      const data = await fetchQueryRun(runId)
      if (epoch !== pollEpoch) return
      run = data
      if (!runExportSupported) await loadRunExportSupport(run)
    } catch {
      // transient failure — keep current view and retry below
    }
    if (epoch === pollEpoch && hasPendingHosts(run) && pollAttempts < maxPollAttempts) {
      pollTimer = setTimeout(pollRun, pollIntervalMs)
    }
  }

  function openHostResults(queryId: string) {
    selectedHostId = queryId
    resultsOpen = true
    loadHostResults(1)
  }

  async function loadHostResults(targetPage: number) {
    hostLoading = true
    try {
      hostResults = await fetchQueryRunHostResults(runId, selectedHostId, { page: targetPage })
    } catch (err) {
      hostResults = {
        columns: [],
        rows: [],
        total: 0,
        page: 1,
        count_per_page: 0,
        page_count: 0,
        error: (err as Error).message || 'Failed to load results'
      }
    } finally {
      hostLoading = false
    }
  }

  async function loadRunExportSupport(value: QueryRun | null) {
    const host = value?.hosts?.find((h) => h.status === 'complete' && (h.row_count ?? 0) > 0)
    if (!host) {
      runExportSupported = false
      return
    }
    try {
      const res = await fetchQueryRunHostResults(runId, host.query_id, { page: 1, countPerPage: 1 })
      runExportSupported = !!res.export_supported
    } catch {
      runExportSupported = false
    }
  }

  $effect(() => {
    if (!resultsDialog) return
    if (resultsOpen && !resultsDialog.open) resultsDialog.showModal()
    else if (!resultsOpen && resultsDialog.open) resultsDialog.close()
  })

  function handleResultsClose() {
    resultsOpen = false
    selectedHostId = ''
    hostResults = null
  }

  function statusVariant(status?: string): 'success' | 'danger' | 'warning' {
    if (status === 'complete') return 'success'
    if (status === 'error') return 'danger'
    return 'warning'
  }

  function statusSummary(value: QueryRun): string {
    const parts: string[] = []
    if (value.complete_count) parts.push(`${value.complete_count} complete`)
    if (value.pending_count) parts.push(`${value.pending_count} pending`)
    if (value.error_count) parts.push(`${value.error_count} error`)
    return parts.length ? parts.join(' · ') : '—'
  }
</script>

<section class="vstack gap-4">
  {#if loading}
    <Spinner fill />
  {:else if run}
    <header class="mb-4">
      <h1 class="mb-2">Query Run</h1>
      <p class="text-light">
        {run.host_count || run.hosts?.length || 0} hosts · {statusSummary(run)} ·
        {formatTimestamp(run.created_at)}
      </p>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <article class="card">
      <code class="query-text">{run.query}</code>
    </article>

    <div class="hstack justify-between">
      <h2>Hosts</h2>
      {#if runDownloadHref}
        <DownloadResultsButton href={runDownloadHref} label="Download all CSV" />
      {/if}
    </div>

    <div class="table">
      <table class="hosts-table">
        <thead>
          <tr>
            <SortableHeader bind:state={hostSort} col="host" label="Host" />
            <SortableHeader bind:state={hostSort} col="user" label="User" />
            <SortableHeader bind:state={hostSort} col="platform" label="Platform" />
            <SortableHeader bind:state={hostSort} col="status" label="Status" />
            <SortableHeader bind:state={hostSort} col="rows" label="Rows" align="right" />
            <SortableHeader bind:state={hostSort} col="completed" label="Completed" />
          </tr>
        </thead>
        <tbody>
          {#each sortedHosts as host}
            <tr
              class="host-row"
              tabindex="0"
              aria-label="Click to show results"
              onclick={() => openHostResults(host.query_id)}
              onkeydown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault()
                  openHostResults(host.query_id)
                }
              }}
            >
              <td><strong>{host.hostname || host.node_uuid}</strong></td>
              <td>
                {#if host.owner_name || host.owner_email}
                  <strong>{host.owner_name || host.owner_email}</strong>
                  {#if host.owner_name && host.owner_email}
                    <p class="text-light">{host.owner_email}</p>
                  {/if}
                {:else}
                  <span class="text-light">Unassigned</span>
                {/if}
              </td>
              <td class="text-light">{host.platform || '—'}</td>
              <td><span class="badge" data-variant={statusVariant(host.status)}>{host.status}</span></td>
              <td class="align-right">{host.status === 'complete' ? host.row_count ?? 0 : '—'}</td>
              <td class="text-light">{host.timestamp ? formatTimestamp(host.timestamp) : '—'}</td>
            </tr>
          {:else}
            <tr><td colspan="6" class="align-center text-light">This run has no hosts</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {:else}
    <ErrorMessage message={error} onClose={() => (error = '')} />
    <article class="card align-center text-light">Query run not found</article>
  {/if}
</section>

<dialog bind:this={resultsDialog} class="host-results" closedby="any" onclose={handleResultsClose}>
  {#if selectedHost}
    <header>
      <h2>{selectedHost.hostname || selectedHost.node_uuid}</h2>
      <p class="hstack gap-2">
        <span class="badge" data-variant={statusVariant(selectedHost.status)}>{selectedHost.status}</span>
        <span class="text-light">{selectedHost.platform || '—'}</span>
        {#if selectedHost.owner_name || selectedHost.owner_email}
          <span class="text-light">· {selectedHost.owner_name || selectedHost.owner_email}</span>
        {/if}
        {#if selectedHost.status === 'complete'}
          <span class="text-light">· {selectedHost.row_count ?? 0} row{selectedHost.row_count === 1 ? '' : 's'}</span>
          {#if hostDownloadHref}
            <DownloadResultsButton href={hostDownloadHref} />
          {/if}
        {/if}
      </p>
    </header>
    <section>
      <code class="query-text">{run?.query}</code>
      <QueryResultTable
        columns={hostResults?.columns ?? []}
        rows={hostResults?.rows ?? []}
        total={hostResults?.total ?? 0}
        page={hostResults?.page ?? 1}
        pageCount={hostResults?.page_count ?? 1}
        loading={hostLoading}
        pending={hostResults?.pending ?? false}
        error={hostResults?.error ?? selectedHost.error ?? ''}
        browsingDisabled={hostResults?.browsing_disabled ?? false}
        onPageChange={(p) => loadHostResults(p)}
      />
    </section>
  {/if}
  <footer class="hstack justify-end">
    <button type="button" class="outline" onclick={() => resultsDialog?.close()}>Close</button>
  </footer>
</dialog>

<style>
  .query-text {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .hosts-table .host-row {
    cursor: pointer;
  }
  .hosts-table .host-row:focus-visible {
    outline: 2px solid var(--primary);
    outline-offset: -2px;
  }
  .host-results {
    width: min(72rem, 94vw);
  }
  .host-results > section {
    max-height: 64vh;
    overflow: auto;
  }
</style>
