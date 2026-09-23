<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import { page } from '$app/state'
  import { pushState, replaceState } from '$app/navigation'
  import {
    createYaraScan,
    createYaraSignatureSource,
    deleteYaraSignatureSource as apiDeleteYaraSignatureSource,
    fetchGroups,
    fetchGroupMachines,
    fetchMachines,
    fetchYaraScanMatches,
    fetchYaraScan,
    fetchYaraScanTargets,
    fetchYaraScans,
    fetchYaraSignatureSources,
    updateYaraSignatureSource,
    type Group,
    type YaraScan,
    type YaraScanMatch,
    type YaraScanTarget,
    type YaraSignatureSource
  } from '$lib/api'
  import ActionsMenu from '$lib/components/ActionsMenu.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import SelectDropdown from '$lib/components/SelectDropdown.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import TextListInput from '$lib/components/TextListInput.svelte'
  import { formatTimestamp, toast } from '$lib/util'
  import { canFrom, me } from '$lib/auth'
  import Plus from '@lucide/svelte/icons/plus'

  type OatTabsElement = HTMLElement & { activeIndex: number }

  const scanCountPerPage = 10
  const matchCountPerPage = 100

  let groups = $state<Group[]>([])
  let sources = $state<YaraSignatureSource[]>([])
  let scans = $state<YaraScan[]>([])
  let matches = $state<YaraScanMatch[]>([])
  let targets = $state<YaraScanTarget[]>([])
  let selectedScan = $state<YaraScan | null>(null)

  const scanPage = $derived(Math.max(1, Number(page.url.searchParams.get('scans')) || 1))
  let scanPageCount = $state(1)
  let scanTotal = $state(0)
  let matchPage = $state(1)
  let matchPageCount = $state(1)
  let matchTotal = $state(0)
  let targetTotal = $state(0)
  let targetPage = $state(1)
  let targetPageCount = $state(1)
  let outcome = $state('matched')
  let resultSearch = $state('')
  let appliedSearch = $state('')
  let targetStatus = $state('')
  let scanSection = $state<'results' | 'targets'>('results')
  let matchesError = $state('')
  let targetsError = $state('')
  let refreshing = $state(false)
  let lastRefreshed = $state('')
  let previewCount = $state<number | null>(null)
  let previewLoading = $state(false)
  let previewError = $state('')
  let scanRequest = 0
  let matchRequest = 0
  let targetRequest = 0
  let selectionRequest = 0
  let selectionLoadingID = ''
  const selectedID = $derived(scanIDFromURL(page.url))
  let paths = $state([''])
  let ruleURLs = $state([''])
  let scanGroupID = $state('')
  let sourceGroupID = $state('')
  let sourceURL = $state('')
  let sourceLabel = $state('')
  let sourceEnabled = $state(true)
  let editingSource = $state<YaraSignatureSource | null>(null)
  let scanDialogOpen = $state(false)
  let sourceDialogOpen = $state(false)
  let deleteOpen = $state(false)
  let selectedSource = $state<YaraSignatureSource | null>(null)

  let loading = $state(true)
  let scansLoading = $state(false)
  let matchesLoading = $state(false)
  let targetsLoading = $state(false)
  let savingSource = $state(false)
  let deletingSource = $state(false)
  let startingScan = $state(false)
  let error = $state('')
  let scanError = $state('')
  let sourceError = $state('')
  const tabSlugs = ['results', 'sources']
  const activeTabIndex = $derived.by(() => {
    const i = tabSlugs.indexOf(page.url.hash.replace(/^#/, ''))
    return i >= 0 ? i : 0
  })
  let tabs = $state<OatTabsElement>()
  let scanDialog = $state<HTMLDialogElement>()
  let sourceDialog = $state<HTMLDialogElement>()

  const groupOptions = $derived([
    { value: '', label: 'All machines' },
    ...groups.map((group) => ({ value: group.uuid, label: group.name || 'Untitled' }))
  ])
  const outcomeOptions = [
    { value: 'matched', label: 'Matched' },
    { value: 'no_match', label: 'Scanned, no match' },
    { value: 'all', label: 'All file results' }
  ]
  const targetStatusOptions = [
    { value: '', label: 'All statuses' },
    { value: 'pending', label: 'Pending dispatch' },
    { value: 'dispatched', label: 'Awaiting response' },
    { value: 'complete', label: 'Responded' },
    { value: 'error', label: 'Error / timeout' }
  ]
  const scanStart = $derived(scanTotal === 0 ? 0 : (scanPage - 1) * scanCountPerPage + 1)
  const scanEnd = $derived(Math.min(scanPage * scanCountPerPage, scanTotal))
  const matchStart = $derived(matchTotal === 0 ? 0 : (matchPage - 1) * matchCountPerPage + 1)
  const matchEnd = $derived(Math.min(matchPage * matchCountPerPage, matchTotal))
  const hasPath = $derived(paths.some((path) => path.trim()))
  const hasRuleURL = $derived(ruleURLs.some((url) => url.trim()))
  const canCreateScan = $derived(canFrom($me, 'yara_scan', 'create'))
  const canCreateSource = $derived(canFrom($me, 'yara_source', 'create'))
  const canUpdateSource = $derived(canFrom($me, 'yara_source', 'update'))
  const canDeleteSource = $derived(canFrom($me, 'yara_source', 'delete'))

  $effect(() => {
    const open = scanDialogOpen
    const group = scanGroupID
    previewCount = null
    previewError = ''
    previewLoading = false
    if (!open) return
    if (!canFrom($me, 'machine', 'view')) {
      previewError = 'Target count requires machine viewing permission.'
      return
    }
    let cancelled = false
    previewLoading = true
    const request = group
      ? fetchGroupMachines(group, { countPerPage: 1 })
      : fetchMachines({ countPerPage: 1 })
    request.then((data) => {
      if (!cancelled) previewCount = data.total_count ?? null
    }).catch((err) => {
      if (!cancelled) previewError = (err as Error).message || 'Could not preview target count'
    }).finally(() => { if (!cancelled) previewLoading = false })
    return () => { cancelled = true }
  })

  onMount(() => {
    void loadAll()
    const syncSelection = () => {
      const id = scanIDFromURL(new URL(window.location.href))
      if (id && id !== selectedScan?.uuid) void loadSelection(id)
      else if (!id && selectedScan) resetSelectionState()
    }
    window.addEventListener('popstate', syncSelection)
    window.addEventListener('hashchange', syncSelection)
    const timer = setInterval(() => {
      if (document.hidden || loading || refreshing || scansLoading || matchesLoading || targetsLoading) return
      if ([selectedScan, ...scans].some((scan) => scan && ['pending', 'running'].includes(scan.status))) {
        void refreshSelected()
      }
    }, 5000)
    return () => {
      clearInterval(timer)
      window.removeEventListener('popstate', syncSelection)
      window.removeEventListener('hashchange', syncSelection)
    }
  })

  $effect(() => {
    if (!tabs || tabs.activeIndex === activeTabIndex) return
    tabs.activeIndex = activeTabIndex
  })

  // Reload when the page params change (pagination links). The first run is the
  // initial mount, which `loadAll`/`selectScan` already cover, so skip it.
  // `untrack` keeps the effect from depending on state read inside the loaders.
  let scanReloadReady = false
  $effect(() => {
    scanPage
    if (!scanReloadReady) {
      scanReloadReady = true
      return
    }
    untrack(() => void loadScans())
  })

  $effect(() => {
    const id = selectedID
    if (loading) return
    untrack(() => {
      if (id && id !== selectedScan?.uuid) void loadSelection(id)
      else if (!id && selectedScan) resetSelectionState()
    })
  })

  function resetMatchesParam() {
    matchPage = 1
  }

  function scanIDFromURL(url: URL) {
    const hash = url.hash.replace(/^#/, '')
    if (/^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/i.test(hash)) return hash
    return url.searchParams.get('scan') || ''
  }

  $effect(() => {
    if (!scanDialogOpen || !scanDialog || scanDialog.open) return
    scanDialog.showModal()
  })

  $effect(() => {
    if (scanDialogOpen || !scanDialog?.open) return
    scanDialog.close()
  })

  $effect(() => {
    if (!sourceDialogOpen || !sourceDialog || sourceDialog.open) return
    sourceDialog.showModal()
  })

  $effect(() => {
    if (sourceDialogOpen || !sourceDialog?.open) return
    sourceDialog.close()
  })

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

  async function loadAll() {
    loading = true
    error = ''
    try {
      const [groupData, sourceData] = await Promise.all([
        fetchGroups({ page: 1, countPerPage: 100 }),
        fetchYaraSignatureSources({ page: 1, countPerPage: 100 })
      ])
      groups = groupData.groups || []
      sources = sourceData.sources || []
      await loadScans(scanPage)
      if (selectedID) await loadSelection(selectedID)
    } catch (err) {
      error = (err as Error).message || 'Failed to load YARA'
    } finally {
      loading = false
    }
  }

  async function loadSources() {
    const data = await fetchYaraSignatureSources({ page: 1, countPerPage: 100 })
    sources = data.sources || []
  }

  async function loadScans(targetPage = scanPage) {
    const request = ++scanRequest
    scansLoading = true
    error = ''
    try {
      const data = await fetchYaraScans({ page: targetPage, countPerPage: scanCountPerPage })
      if (request !== scanRequest) return
      scans = data.scans || []
      scanPageCount = data.page_count || 1
      scanTotal = data.total_count || scans.length
    } catch (err) {
      if (request === scanRequest) error = (err as Error).message || 'Failed to load scans'
    } finally {
      if (request === scanRequest) scansLoading = false
    }
  }

  async function loadMatches(pageNumber = matchPage) {
    if (!selectedScan) {
      matches = []
      matchTotal = 0
      return
    }
    const id = selectedScan.uuid
    const request = ++matchRequest
    matchesLoading = true
    matchesError = ''
    try {
      const data = await fetchYaraScanMatches(id, { page: pageNumber, countPerPage: matchCountPerPage, outcome, query: appliedSearch })
      if (request !== matchRequest || selectedScan?.uuid !== id) return
      if (!data.matches?.length && pageNumber > 1) {
        matchPage = 1
        await loadMatches(1)
        return
      }
      matches = data.matches || []
      matchPageCount = data.page_count || 1
      matchTotal = data.total_count || matches.length
    } catch (err) {
      if (request === matchRequest) matchesError = (err as Error).message || 'Failed to load file results'
    } finally {
      if (request === matchRequest) matchesLoading = false
    }
  }

  async function loadTargets() {
    if (!selectedScan) {
      targets = []
      targetTotal = 0
      return
    }
    const id = selectedScan.uuid
    const request = ++targetRequest
    targetsLoading = true
    targetsError = ''
    try {
      const data = await fetchYaraScanTargets(id, { page: targetPage, countPerPage: 100, status: targetStatus })
      if (request !== targetRequest || selectedScan?.uuid !== id) return
      if (!data.targets?.length && targetPage > 1) {
        targetPage = 1
        await loadTargets()
        return
      }
      targets = data.targets || []
      targetTotal = data.total_count || targets.length
      targetPageCount = data.page_count || 1
    } catch (err) {
      if (request === targetRequest) targetsError = (err as Error).message || 'Failed to load targets'
    } finally {
      if (request === targetRequest) targetsLoading = false
    }
  }

  async function startScan() {
    if (!canCreateScan) return
    const scanPaths = paths.map((path) => path.trim()).filter(Boolean)
    if (scanPaths.length === 0) {
      scanError = 'At least one path is required'
      return
    }
    const urls = ruleURLs.map((url) => url.trim()).filter(Boolean)
    if (urls.length === 0) {
      scanError = 'At least one rule URL is required'
      return
    }
    for (const url of urls) {
      try {
        if (new URL(url).protocol !== 'https:') throw new Error('HTTPS required')
      } catch {
        scanError = `Use a valid HTTPS rule URL: ${url}`
        return
      }
    }
    startingScan = true
    scanError = ''
    try {
      const scan = await createYaraScan({ paths: scanPaths, group_id: scanGroupID, rule_urls: urls })
      resetScanForm()
      scanDialogOpen = false
      const url = new URL(page.url)
      url.searchParams.delete('scans')
      replaceState(url, {})
      selectScan(scan)
      await loadScans(1)
    } catch (err) {
      scanError = (err as Error).message || 'Failed to start scan'
    } finally {
      startingScan = false
    }
  }

  function openStartScan() {
    if (!canCreateScan) return
    resetScanForm()
    scanDialogOpen = true
  }

  function runAgain() {
    if (!selectedScan || !canCreateScan) return
    paths = [...selectedScan.paths]
    ruleURLs = [...(selectedScan.rule_urls || [])]
    scanGroupID = selectedScan.group_id || ''
    scanError = ''
    scanDialogOpen = true
  }

  function resetScanForm() {
    paths = ['']
    ruleURLs = ['']
    scanGroupID = ''
    scanError = ''
  }

  function handleScanDialogClose() {
    scanDialogOpen = false
    if (!startingScan) {
      resetScanForm()
    }
  }

  function openCreateSource() {
    if (!canCreateSource) return
    resetSourceForm()
    sourceDialogOpen = true
  }

  function openEditSource(source: YaraSignatureSource) {
    if (!canUpdateSource) return
    editingSource = source
    sourceGroupID = source.group_id || ''
    sourceURL = source.url || ''
    sourceLabel = source.label || ''
    sourceEnabled = source.enabled !== false
    sourceDialogOpen = true
  }

  function resetSourceForm() {
    editingSource = null
    sourceGroupID = ''
    sourceURL = ''
    sourceLabel = ''
    sourceEnabled = true
    sourceError = ''
  }

  async function saveSource() {
    if (editingSource ? !canUpdateSource : !canCreateSource) return
    if (!sourceURL.trim()) {
      sourceError = 'URL is required'
      return
    }
    savingSource = true
    sourceError = ''
    try {
      const payload = {
        group_id: sourceGroupID,
        url: sourceURL.trim(),
        label: sourceLabel.trim(),
        enabled: sourceEnabled
      }
      if (editingSource) {
        await updateYaraSignatureSource(editingSource.uuid, payload)
      } else {
        await createYaraSignatureSource(payload)
      }
      resetSourceForm()
      sourceDialogOpen = false
      await loadSources()
    } catch (err) {
      sourceError = (err as Error).message || 'Failed to save source config'
    } finally {
      savingSource = false
    }
  }

  function handleSourceDialogClose() {
    sourceDialogOpen = false
    if (!savingSource) {
      resetSourceForm()
    }
  }

  function confirmDelete(source: YaraSignatureSource) {
    if (!canDeleteSource) return
    selectedSource = source
    deleteOpen = true
  }

  async function deleteSource() {
    if (!selectedSource) return
    deletingSource = true
    error = ''
    try {
      await apiDeleteYaraSignatureSource(selectedSource.uuid)
      deleteOpen = false
      selectedSource = null
      await loadSources()
    } catch (err) {
      error = (err as Error).message || 'Failed to delete allowlist URL'
    } finally {
      deletingSource = false
    }
  }

  function selectScan(scan: YaraScan, navigate = true) {
    selectionRequest++
    matchRequest++
    targetRequest++
    selectedScan = scan
    matches = []
    targets = []
    matchTotal = 0
    targetTotal = 0
    matchPageCount = 1
    targetPageCount = 1
    targetPage = 1
    outcome = 'matched'
    resultSearch = ''
    appliedSearch = ''
    targetStatus = ''
    scanSection = 'results'
    lastRefreshed = ''
    void loadTargets()
    resetMatchesParam()
    void loadMatches(1)
    if (navigate) {
      const url = new URL(page.url)
      url.searchParams.delete('scan')
      url.searchParams.delete('matches')
      url.hash = scan.uuid
      pushState(url, {})
    }
  }

  async function loadSelection(id: string) {
    if (selectionLoadingID === id) return
    selectionLoadingID = id
    const request = ++selectionRequest
    try {
      const scan = await fetchYaraScan(id)
      if (request === selectionRequest) {
        selectScan(scan, false)
        if (page.url.searchParams.has('scan')) {
          const url = new URL(page.url)
          url.searchParams.delete('scan')
          url.hash = scan.uuid
          replaceState(url, {})
        }
      }
    } catch (err) {
      if (request === selectionRequest) error = (err as Error).message || 'Failed to load selected scan'
    } finally {
      if (selectionLoadingID === id) selectionLoadingID = ''
    }
  }

  function handleScanLink(event: MouseEvent, scan: YaraScan) {
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
    selectScan(scan, false)
  }

  function filterResults() {
    appliedSearch = resultSearch.trim()
    matchPage = 1
    matches = []
    void loadMatches()
  }

  function filterTargets() {
    targetPage = 1
    targets = []
    void loadTargets()
  }

  function resetSelectionState() {
    selectionRequest++
    matchRequest++
    targetRequest++
    selectedScan = null
    matches = []
    targets = []
  }

  function clearSelection() {
    resetSelectionState()
    const url = new URL(page.url)
    url.searchParams.delete('scan')
    url.hash = 'results'
    pushState(url, {})
  }

  function targetEvidence(target: YaraScanTarget) {
    if (target.matched_result_count > 0) return 'Matched'
    if (target.result_count > 0) return 'Scanned, no match'
    if (target.status === 'complete') return 'No scan evidence'
    if (target.status === 'error') return 'No scan evidence'
    return 'Awaiting results'
  }

  async function copyText(value: string, label: string) {
    try {
      await navigator.clipboard.writeText(value)
      toast(`${label} copied`, 'Clipboard', { variant: 'success' })
    } catch {
      toast(`Could not copy ${label.toLowerCase()}`, 'Clipboard', { variant: 'danger' })
    }
  }

  function generatedSQL(scan: YaraScan) {
    const quote = (text: string) => text.replaceAll("'", "''")
    const conditions = scan.paths.map((path) => `path LIKE '${quote(path)}'`).join(' OR ')
    return (scan.rule_urls || []).map((url) => `SELECT path, matches, count, sigurl FROM yara_file WHERE (${conditions}) AND sigurl = '${quote(url)}'`).join('\nUNION ALL\n') + ';'
  }

  function scopeLabel(source: YaraSignatureSource) {
    return source.group_name || 'All machines'
  }

  function scanGroupLabel(scan: YaraScan) {
    return scan.group_name || 'All machines'
  }

  async function refreshSelected() {
    if (refreshing) return
    refreshing = true
    const id = selectedScan?.uuid
    try {
      await loadScans(scanPage)
      if (id) {
        const scan = await fetchYaraScan(id)
        if (selectedScan?.uuid !== id) return
        selectedScan = scan
        await Promise.all([loadTargets(), loadMatches()])
      }
      if (!error && !matchesError && !targetsError) lastRefreshed = new Date().toISOString()
    } catch (err) {
      error = (err as Error).message || 'Refresh failed; showing previously loaded results'
    } finally { refreshing = false }
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">YARA</h1>
      <p class="text-light">Run on-demand signature scans across machines</p>
    </div>
    {#if canCreateScan}
      <button type="button" onclick={openStartScan}>Start Scan</button>
    {/if}
  </header>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if loading}
    <Spinner fill />
  {:else}
    <ot-tabs bind:this={tabs} class="yara-tabs" onot-tab-change={handleTabChange}>
      <div role="tablist" aria-label="YARA sections" style:display={selectedScan ? 'none' : undefined}>
        <button
          type="button"
          role="tab"
          aria-selected={activeTabIndex === 0}
          onclick={() => setTab(0)}
        >
          Results
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activeTabIndex === 1}
          onclick={() => setTab(1)}
        >
          Source Config
        </button>
      </div>
      {#if selectedScan}
        <div class="text-right"><button type="button" class="ghost" onclick={clearSelection}>&larr; Back to scans</button></div>
      {/if}

      <div role="tabpanel">
        <div class="vstack gap-4">
          {#if !selectedScan}
          <section class="vstack gap-3">
            <div class="hstack justify-between">
              <h2>Scans</h2>
              <button
                type="button"
                class="small outline"
                disabled={refreshing || scansLoading || targetsLoading || matchesLoading}
                onclick={refreshSelected}
              >
                {refreshing ? 'Refreshing…' : 'Refresh'}
              </button>
            </div>
          {#if scansLoading && !scans.length}
            <Spinner />
          {:else}
            <div class="table">
              <table class="scans-table">
                <thead>
                  <tr>
                    <th>Scan</th>
                    <th>Created</th>
                    <th>Paths</th>
                    <th>Group</th>
                    <th>Status</th>
                    <th class="align-right">Finished targets</th>
                    <th class="align-right">Positive results</th>
                  </tr>
                </thead>
                <tbody>
                  {#each scans as scan}
                    <tr>
                      <td>
                        <a href={`#${scan.uuid}`} class="cell-link" aria-label={`View scan ${scan.uuid}`} onclick={(event) => handleScanLink(event, scan)}>{scan.uuid.slice(0, 8)}</a>
                      </td>
                      <td>{formatTimestamp(scan.created_at)}</td>
                      <td>{#each scan.paths as path}<div><code>{path}</code></div>{/each}</td>
                      <td>{scanGroupLabel(scan)}</td>
                      <td>{scan.status}</td>
                      <td class="align-right">{scan.completed_count || 0} / {scan.target_count || 0}</td>
                      <td class="align-right">{scan.match_count || 0}</td>
                    </tr>
                  {:else}
                    <tr><td colspan="7" class="align-center text-light">No scans yet</td></tr>
                  {/each}
                </tbody>
              </table>
            </div>
            <footer class="hstack justify-between">
              <p class="text-light">
                Showing <strong>{scanStart}</strong> to <strong>{scanEnd}</strong> of <strong>{scanTotal}</strong> scans
              </p>
              <Pagination currentPage={scanPage} pageCount={scanPageCount} param="scans" />
            </footer>
          {/if}
          </section>
          {/if}

          {#if selectedScan}
            <section class="card vstack gap-3" aria-label="Selected scan details">
              <header class="hstack justify-between">
                <div>
                  <div class="hstack gap-2">
                    <h2>Scan {selectedScan.uuid.slice(0, 8)}</h2>
                    <span class="badge outline">{selectedScan.status}</span>
                  </div>
                  <p class="text-light">Started {formatTimestamp(selectedScan.created_at)}</p>
                </div>
                <div class="hstack gap-2">
                  {#if canCreateScan}<button type="button" class="small outline" onclick={runAgain}>Run again</button>{/if}
                  <button type="button" class="small outline" disabled={refreshing} onclick={refreshSelected}>{refreshing ? 'Refreshing…' : 'Refresh'}</button>
                </div>
              </header>
              <p class="text-light">
                {scanGroupLabel(selectedScan)} · {selectedScan.completed_count || 0} of {selectedScan.target_count || 0} finished · {selectedScan.match_count || 0} matches
              </p>
              {#if (selectedScan.target_count || 0) > 0}
                <progress class="w-100" aria-label="Finished targets" value={selectedScan.completed_count || 0} max={selectedScan.target_count}></progress>
              {/if}
              {#if selectedScan.error}
                <details>
                  <summary>Scan error</summary>
                  <p>{selectedScan.error}</p>
                  <button type="button" class="small outline" onclick={() => copyText(selectedScan!.error || '', 'Error')}>Copy error</button>
                </details>
              {/if}
              <details>
                <summary>Scan configuration</summary>
                <div class="row mt-4">
                  <section class="col-6 vstack gap-2" aria-labelledby="scan-paths-heading">
                    <h3 id="scan-paths-heading">Paths</h3>
                    {#each selectedScan.paths as path}<code>{path}</code>{/each}
                  </section>
                  <section class="col-6 vstack gap-2" aria-labelledby="scan-rules-heading">
                    <h3 id="scan-rules-heading">Rule URLs</h3>
                    {#each selectedScan.rule_urls || [] as url}<code>{url}</code>{/each}
                  </section>
                </div>
                <details class="mt-4">
                  <summary>Diagnostic SQL</summary>
                  <p class="text-light">Includes zero-match results. Older scans may have been dispatched with a positive-match filter.</p>
                  <pre>{generatedSQL(selectedScan)}</pre>
                  <button type="button" class="small outline" onclick={() => copyText(generatedSQL(selectedScan!), 'SQL')}>Copy SQL</button>
                </details>
              </details>
              <p class="text-light">
                {#if ['pending', 'running'].includes(selectedScan.status)}Refreshes every 5 seconds while this page is visible.{/if}
                {#if lastRefreshed}Last refreshed {formatTimestamp(lastRefreshed)}.{/if}
              </p>
            </section>

            <div role="tablist" aria-label="Selected scan data">
              <button type="button" role="tab" aria-selected={scanSection === 'results'} onclick={() => (scanSection = 'results')}>File results</button>
              <button type="button" role="tab" aria-selected={scanSection === 'targets'} onclick={() => (scanSection = 'targets')}>Targets</button>
            </div>

            {#if scanSection === 'results'}
            <section class="vstack gap-3">
            <header>
                <h2>File results</h2>
            </header>
            <form class="row filter-bar" onsubmit={(event) => { event.preventDefault(); filterResults() }}>
              <div class="col-4"><SelectDropdown label="Outcome" options={outcomeOptions} bind:value={outcome} /></div>
              <label class="col-6">Search results<input type="search" placeholder="Machine, path, rule, or URL" bind:value={resultSearch} maxlength={500} /></label>
              <button type="submit" class="col-2">Apply filters</button>
            </form>
            <ErrorMessage message={matchesError} onClose={() => (matchesError = '')} />
            <div class="table">
              <table class="matches-table">
                <thead>
                  <tr>
                    <th>Machine</th>
                    <th>Path</th>
                    <th>Outcome</th>
                    <th>Matching rules</th>
                    <th class="align-right">Rule count</th>
                    <th>Rule URL / reported</th>
                  </tr>
                </thead>
                <tbody>
                  {#if matchesLoading && !matches.length}
                    <tr><td colspan="6" class="align-center text-light">Loading file results...</td></tr>
                  {:else}
                    {#each matches as match}
                      <tr>
                        <td>{match.hostname || match.machine_uuid}</td>
                        <td><code>{match.path}</code></td>
                        <td><span class="badge outline">{match.count ? 'Matched' : 'Scanned, no match'}</span></td>
                        <td>{match.matches || '—'}</td>
                        <td class="align-right">{match.count || 0}</td>
                        <td><div>{match.sigurl || 'Source URL not recorded'}</div><small class="text-light">{formatTimestamp(match.created_at)}</small></td>
                      </tr>
                    {:else}
                      <tr><td colspan="6" class="align-center text-light">{matchesError ? 'File results could not be loaded.' : 'No file results for these filters. Check Targets for execution status and scan evidence.'}</td></tr>
                    {/each}
                  {/if}
                </tbody>
              </table>
            </div>
            <footer class="hstack justify-between">
              <p class="text-light">
                Showing <strong>{matchStart}</strong> to <strong>{matchEnd}</strong> of <strong>{matchTotal}</strong> file results
              </p>
              <Pagination currentPage={matchPage} pageCount={matchPageCount} disabled={matchesLoading} label="File results pagination" onPageChange={(value) => { matchPage = value; void loadMatches() }} />
            </footer>
          </section>

          {:else}
          <section class="vstack gap-3">
            <h2>Targets</h2>
            <p class="text-light">Execution status is separate from evidence. No scan evidence does not mean a file is absent: check paths, access, rule loading, and endpoint logs. Older scans did not retain zero-match results.</p>
            <form class="row filter-bar" onsubmit={(event) => { event.preventDefault(); filterTargets() }}>
              <div class="col-4"><SelectDropdown label="Execution status" options={targetStatusOptions} bind:value={targetStatus} /></div>
              <button type="submit" class="col-2">Apply filter</button>
            </form>
            <ErrorMessage message={targetsError} onClose={() => (targetsError = '')} />
            <div class="table">
              <table class="targets-table">
                <thead>
                  <tr>
                    <th>Machine</th>
                    <th>Status</th>
                    <th>Scan evidence</th>
                    <th>Dispatched</th>
                    <th>Completed</th>
                    <th>Error</th>
                  </tr>
                </thead>
                <tbody>
                  {#if targetsLoading && !targets.length}
                    <tr><td colspan="6" class="align-center text-light">Loading targets...</td></tr>
                  {:else}
                    {#each targets as target}
                      <tr>
                        <td>{target.hostname || target.machine_uuid}</td>
                        <td>{target.status === 'complete' ? 'Responded' : target.status === 'pending' ? 'Pending dispatch' : target.status === 'dispatched' ? 'Awaiting response' : 'Error / timeout'}</td>
                        <td>{targetEvidence(target)}{#if target.result_count > 0}<small class="text-light"> · {target.result_count} file/source results, {target.matched_result_count} positive</small>{/if}</td>
                        <td>{formatTimestamp(target.dispatched_at)}</td>
                        <td>{formatTimestamp(target.completed_at)}</td>
                        <td>{#if target.error}<details><summary>View error</summary><p>{target.error}</p><button type="button" class="small outline" onclick={() => copyText(target.error || '', 'Error')}>Copy error</button></details>{:else}—{/if}</td>
                      </tr>
                    {:else}
                      <tr><td colspan="6" class="align-center text-light">{targetsError ? 'Targets could not be loaded.' : 'No targets for this filter'}</td></tr>
                    {/each}
                  {/if}
                </tbody>
              </table>
            </div>
            <footer class="hstack justify-between">
              <p class="text-light"><strong>{targetTotal}</strong> targets</p>
              <Pagination currentPage={targetPage} pageCount={targetPageCount} label="Targets pagination" disabled={targetsLoading} onPageChange={(value) => { targetPage = value; void loadTargets() }} />
            </footer>
          </section>
          {/if}
          {/if}
        </div>
      </div>

      <div role="tabpanel">
        <section class="vstack gap-3">
          <div class="hstack justify-between">
            <p class="text-light">{sources.length} source configs</p>
            {#if canCreateSource}
              <button type="button" class="gap-1" onclick={openCreateSource}>
                <Plus size={16} aria-hidden="true" />
                Add Source Config
              </button>
            {/if}
          </div>

          <div class="table">
            <table class="sources-table">
              <thead>
                <tr>
                  <th>Scope</th>
                  <th>Label</th>
                  <th>URL</th>
                  <th>Enabled</th>
                  <th class="align-right"><span class="sr-only">Actions</span></th>
                </tr>
              </thead>
              <tbody>
                {#each sources as source}
                  <tr>
                    <td>{scopeLabel(source)}</td>
                    <td>{source.label || ''}</td>
                    <td title={source.url}>
                      {#if canUpdateSource}
                        <button type="button" class="cell-link" onclick={() => openEditSource(source)}>
                          {source.url}
                        </button>
                      {:else}
                        <strong>{source.url}</strong>
                      {/if}
                    </td>
                    <td>{source.enabled === false ? 'No' : 'Yes'}</td>
                    <td class="align-right">
                      <ActionsMenu label={`Actions for ${source.label || source.url}`}>
                          <button role="menuitem" type="button" onclick={() => copyText(source.url, 'Source URL')}>Copy URL</button>
                          {#if canUpdateSource}
                            <button role="menuitem" type="button" onclick={() => openEditSource(source)}>Edit</button>
                          {/if}
                          {#if canUpdateSource && canDeleteSource}<hr />{/if}
                          {#if canDeleteSource}
                            <button role="menuitem" type="button" onclick={() => confirmDelete(source)}>Delete</button>
                          {/if}
                      </ActionsMenu>
                    </td>
                  </tr>
                {:else}
                  <tr><td colspan="5" class="align-center text-light">No source configs configured</td></tr>
                {/each}
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </ot-tabs>
  {/if}
</section>

<dialog bind:this={scanDialog} onclose={handleScanDialogClose} closedby="any">
  <form onsubmit={(event) => { event.preventDefault(); startScan() }}>
    <header>
      <h2>Start Scan</h2>
    </header>

    <ErrorMessage message={scanError} onClose={() => (scanError = '')} />

    <div class="vstack">
      <SelectDropdown label="Targets" options={groupOptions} bind:value={scanGroupID} disabled={startingScan} />
      <TextListInput
        label="Paths"
        bind:value={paths}
        placeholder="/var/tmp/%%"
        addLabel="Add path"
        disabled={startingScan}
        required
        splitPaste
      />

      <small data-hint>Use % for one directory level and %% for recursive matching. Paths are case-sensitive. Paste multiple paths on separate lines.</small>

      <TextListInput
        label="Rule URLs"
        bind:value={ruleURLs}
        placeholder="https://rules.example.com/rule.yar"
        addLabel="Add rule URL"
        disabled={startingScan}
        required
        splitPaste
      />
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => scanDialog?.close()}>Cancel</button>
      <button
        type="submit"
        class="gap-1"
        disabled={startingScan || !hasPath || !hasRuleURL || previewCount === 0}
        aria-busy={startingScan ? 'true' : undefined}
        data-spinner="small"
      >
        {startingScan ? 'Starting...' : 'Start'}
      </button>
    </footer>
  </form>
</dialog>

<dialog bind:this={sourceDialog} onclose={handleSourceDialogClose} closedby="any">
  <form onsubmit={(event) => { event.preventDefault(); saveSource() }}>
    <header>
      <h2>{editingSource ? 'Edit Source Config' : 'Add Source Config'}</h2>
    </header>

    <ErrorMessage message={sourceError} onClose={() => (sourceError = '')} />

    <div class="vstack">
      <SelectDropdown label="Scope" options={groupOptions} bind:value={sourceGroupID} disabled={savingSource} />

      <label data-field>
        URL <span class="req" aria-hidden="true">*</span>
        <input bind:value={sourceURL} placeholder="https://rules.example.com/.*" disabled={savingSource} />
        <small data-hint>Allowlist URL pattern for endpoints. A scan needs a concrete rule URL covered by this pattern. Endpoints receive changes on their next configuration refresh.</small>
      </label>

      <label data-field>
        Label
        <input bind:value={sourceLabel} disabled={savingSource} />
      </label>

      <label class="checkbox-label">
        <input type="checkbox" bind:checked={sourceEnabled} disabled={savingSource} />
        <span>Enabled</span>
      </label>
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => sourceDialog?.close()}>Cancel</button>
      <button type="submit" class="gap-1" disabled={savingSource} aria-busy={savingSource ? 'true' : undefined} data-spinner="small">
        {savingSource ? 'Saving...' : editingSource ? 'Update' : 'Add'}
      </button>
    </footer>
  </form>
</dialog>

<ConfirmDialog
  bind:open={deleteOpen}
  title="Delete Allowlist URL"
  message="Are you sure you want to delete this YARA allowlist URL? Existing scan history will keep its frozen rule URL."
  confirming={deletingSource}
  confirmingLabel="Deleting..."
  onConfirm={deleteSource}
  onCancel={() => (selectedSource = null)}
/>

<style>
  .yara-tabs {
    display: block;
  }
  .checkbox-label {
    align-items: center;
    display: flex;
    gap: var(--space-2);
    min-height: 2.4rem;
  }
  .filter-bar {
    align-items: end;
  }
  .sources-table,
  .scans-table,
  .targets-table,
  .matches-table {
    width: 100%;
  }
  .sources-table td,
  .scans-table td,
  .targets-table td,
  .matches-table td {
    white-space: normal;
    overflow-wrap: anywhere;
    vertical-align: top;
    max-width: 28rem;
    min-width: 7rem;
  }
  code, pre {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .sources-table .cell-link {
    white-space: normal;
    overflow-wrap: anywhere;
    text-align: left;
  }
  .scans-table td:nth-child(2),
  .targets-table td:nth-child(4),
  .targets-table td:nth-child(5) {
    white-space: nowrap;
  }
  .scans-table td:nth-child(3),
  .matches-table td:nth-child(2),
  .targets-table td:last-child {
    min-width: 16rem;
  }
</style>
