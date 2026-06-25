<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import { page } from '$app/state'
  import { fetchQueryRun, fetchQueryRunHostResults, type QueryRun, type AdHocQueryResults } from '$lib/api'
  import { formatTimestamp } from '$lib/util'
  import QueryResultTable from '$lib/components/QueryResultTable.svelte'
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
  let pollTimer: ReturnType<typeof setTimeout> | null = null
  let pollAttempts = 0
  let pollEpoch = 0

  const runId = $derived(page.params.id as string)
  // Resolve the open host from the live run so polling updates flow into the modal.
  const selectedHost = $derived(run?.hosts?.find((h) => h.query_id === selectedHostId) || null)

  onMount(load)
  onDestroy(stopPolling)

  async function load() {
    loading = true
    error = ''
    try {
      run = await fetchQueryRun(runId)
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

    <div class="table">
      <table class="hosts-table">
        <thead>
          <tr>
            <th>Host</th>
            <th>Platform</th>
            <th>Status</th>
            <th>Rows</th>
            <th>Completed</th>
          </tr>
        </thead>
        <tbody>
          {#each run.hosts || [] as host}
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
              <td class="text-light">{host.platform || '—'}</td>
              <td><span class="badge" data-variant={statusVariant(host.status)}>{host.status}</span></td>
              <td>{host.status === 'complete' ? host.row_count ?? 0 : '—'}</td>
              <td class="text-light">{host.timestamp ? formatTimestamp(host.timestamp) : '—'}</td>
            </tr>
          {:else}
            <tr><td colspan="5" class="align-center text-light">This run has no hosts</td></tr>
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
        {#if selectedHost.status === 'complete'}
          <span class="text-light">· {selectedHost.row_count ?? 0} row{selectedHost.row_count === 1 ? '' : 's'}</span>
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
