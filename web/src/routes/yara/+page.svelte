<script lang="ts">
  import { onMount } from 'svelte'
  import {
    createYaraScan,
    createYaraSignatureSource,
    deleteYaraSignatureSource as apiDeleteYaraSignatureSource,
    fetchGroups,
    fetchYaraScanMatches,
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
  import { formatTimestamp } from '$lib/util'

  type OatTabsElement = HTMLElement & { activeIndex: number }

  const scanCountPerPage = 10
  const matchCountPerPage = 100

  let groups: Group[] = []
  let sources: YaraSignatureSource[] = []
  let scans: YaraScan[] = []
  let matches: YaraScanMatch[] = []
  let targets: YaraScanTarget[] = []
  let selectedScan: YaraScan | null = null

  let scanPage = 1
  let scanPageCount = 1
  let scanTotal = 0
  let matchPage = 1
  let matchPageCount = 1
  let matchTotal = 0
  let targetTotal = 0

  let paths = ['']
  let ruleURLs = ['']
  let scanGroupID = ''
  let sourceGroupID = ''
  let sourceURL = ''
  let sourceLabel = ''
  let sourceEnabled = true
  let editingSource: YaraSignatureSource | null = null
  let scanDialogOpen = false
  let sourceDialogOpen = false
  let deleteOpen = false
  let selectedSource: YaraSignatureSource | null = null

  let loading = true
  let scansLoading = false
  let matchesLoading = false
  let targetsLoading = false
  let savingSource = false
  let deletingSource = false
  let startingScan = false
  let error = ''
  let scanError = ''
  let sourceError = ''
  let activeTabIndex = 0
  let tabs: OatTabsElement
  let scanDialog: HTMLDialogElement
  let sourceDialog: HTMLDialogElement

  $: groupOptions = [
    { value: '', label: 'All machines' },
    ...groups.map((group) => ({ value: group.uuid, label: group.name || 'Untitled' }))
  ]
  $: scanStart = scanTotal === 0 ? 0 : (scanPage - 1) * scanCountPerPage + 1
  $: scanEnd = Math.min(scanPage * scanCountPerPage, scanTotal)
  $: matchStart = matchTotal === 0 ? 0 : (matchPage - 1) * matchCountPerPage + 1
  $: matchEnd = Math.min(matchPage * matchCountPerPage, matchTotal)
  $: hasPath = paths.some((path) => path.trim())
  $: hasRuleURL = ruleURLs.some((url) => url.trim())

  onMount(loadAll)

  $: if (tabs && tabs.activeIndex !== activeTabIndex) {
    tabs.activeIndex = activeTabIndex
  }

  $: if (scanDialogOpen && scanDialog && !scanDialog.open) {
    scanDialog.showModal()
  }

  $: if (!scanDialogOpen && scanDialog?.open) {
    scanDialog.close()
  }

  $: if (sourceDialogOpen && sourceDialog && !sourceDialog.open) {
    sourceDialog.showModal()
  }

  $: if (!sourceDialogOpen && sourceDialog?.open) {
    sourceDialog.close()
  }

  function handleTabChange(event: CustomEvent<{ index: number }>) {
    activeTabIndex = event.detail.index
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
      await loadScans(1)
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

  async function loadScans(page = scanPage) {
    scansLoading = true
    error = ''
    try {
      const data = await fetchYaraScans({ page, countPerPage: scanCountPerPage })
      scans = data.scans || []
      scanPage = page
      scanPageCount = data.page_count || 1
      scanTotal = data.total_count || scans.length
      if (selectedScan) {
        selectedScan = scans.find((scan) => scan.uuid === selectedScan?.uuid) || selectedScan
      } else if (scans.length > 0) {
        selectScan(scans[0], false)
      }
    } catch (err) {
      error = (err as Error).message || 'Failed to load scans'
    } finally {
      scansLoading = false
    }
  }

  async function loadMatches(page = matchPage) {
    if (!selectedScan) {
      matches = []
      matchTotal = 0
      return
    }
    matchesLoading = true
    error = ''
    try {
      const data = await fetchYaraScanMatches(selectedScan.uuid, { page, countPerPage: matchCountPerPage })
      matches = data.matches || []
      matchPage = page
      matchPageCount = data.page_count || 1
      matchTotal = data.total_count || matches.length
    } catch (err) {
      error = (err as Error).message || 'Failed to load matches'
    } finally {
      matchesLoading = false
    }
  }

  async function loadTargets() {
    if (!selectedScan) {
      targets = []
      targetTotal = 0
      return
    }
    targetsLoading = true
    error = ''
    try {
      const data = await fetchYaraScanTargets(selectedScan.uuid)
      targets = data.targets || []
      targetTotal = data.total_count || targets.length
    } catch (err) {
      error = (err as Error).message || 'Failed to load targets'
    } finally {
      targetsLoading = false
    }
  }

  async function startScan() {
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
    startingScan = true
    scanError = ''
    try {
      const scan = await createYaraScan({ paths: scanPaths, group_id: scanGroupID, rule_urls: urls })
      resetScanForm()
      scanDialogOpen = false
      selectedScan = scan
      await loadScans(1)
      await loadTargets()
      await loadMatches(1)
      activeTabIndex = 0
    } catch (err) {
      scanError = (err as Error).message || 'Failed to start scan'
    } finally {
      startingScan = false
    }
  }

  function openStartScan() {
    resetScanForm()
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
    resetSourceForm()
    sourceDialogOpen = true
  }

  function openEditSource(source: YaraSignatureSource) {
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

  function selectScan(scan: YaraScan, showResults = true) {
    selectedScan = scan
    loadTargets()
    loadMatches(1)
    if (showResults) {
      activeTabIndex = 0
    }
  }

  function scopeLabel(source: YaraSignatureSource) {
    return source.group_name || 'All machines'
  }

  function scanGroupLabel(scan: YaraScan) {
    return scan.group_name || 'All machines'
  }

  function scanPathsLabel(scan: YaraScan) {
    return (scan.paths || []).join(', ')
  }

  async function refreshSelected() {
    await loadScans(scanPage)
    await loadTargets()
    await loadMatches(matchPage)
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">YARA</h1>
      <p class="text-light">Run on-demand signature scans across machines</p>
    </div>
    <button type="button" onclick={openStartScan}>Start Scan</button>
  </header>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if loading}
    <Spinner />
  {:else}
    <ot-tabs bind:this={tabs} class="yara-tabs" onot-tab-change={handleTabChange}>
      <div role="tablist" aria-label="YARA sections">
        <button
          type="button"
          role="tab"
          aria-selected={activeTabIndex === 0}
          onclick={() => (activeTabIndex = 0)}
        >
          Results
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activeTabIndex === 1}
          onclick={() => (activeTabIndex = 1)}
        >
          Source Config
        </button>
      </div>

      <div role="tabpanel">
        <div class="vstack gap-4">
          <section class="vstack gap-3">
            <div class="hstack justify-between">
              <h2>Scans</h2>
              <button
                type="button"
                class="small outline"
                disabled={scansLoading || targetsLoading || matchesLoading}
                onclick={refreshSelected}
              >
                Refresh
              </button>
            </div>
          {#if scansLoading}
            <Spinner />
          {:else}
            <div class="table">
              <table class="scans-table">
                <thead>
                  <tr>
                    <th>Scan UUID</th>
                    <th>Created</th>
                    <th>Paths</th>
                    <th>Group</th>
                    <th>Status</th>
                    <th>Error</th>
                    <th class="align-right">Targets</th>
                    <th class="align-right">Matches</th>
                  </tr>
                </thead>
                <tbody>
                  {#each scans as scan}
                    <tr aria-current={selectedScan?.uuid === scan.uuid ? 'true' : undefined}>
                      <td>
                        <button type="button" class="cell-link" onclick={() => selectScan(scan)}>{scan.uuid}</button>
                      </td>
                      <td>{formatTimestamp(scan.created_at)}</td>
                      <td title={scanPathsLabel(scan)}>{scanPathsLabel(scan)}</td>
                      <td>{scanGroupLabel(scan)}</td>
                      <td>{scan.status}</td>
                      <td title={scan.error || ''}>{scan.error || ''}</td>
                      <td class="align-right">{scan.completed_count || 0} / {scan.target_count || 0}</td>
                      <td class="align-right">{scan.match_count || 0}</td>
                    </tr>
                  {:else}
                    <tr><td colspan="8" class="align-center text-light">No scans yet</td></tr>
                  {/each}
                </tbody>
              </table>
            </div>
            <footer class="hstack justify-between">
              <p class="text-light">
                Showing <strong>{scanStart}</strong> to <strong>{scanEnd}</strong> of <strong>{scanTotal}</strong> scans
              </p>
              <Pagination currentPage={scanPage} pageCount={scanPageCount} onPageChange={loadScans} />
            </footer>
          {/if}
          </section>

          <section class="vstack gap-3">
            <header class="hstack justify-between">
              <div>
                <h2>Matches</h2>
                {#if selectedScan}
                  <p class="text-light">{selectedScan.uuid}</p>
                {/if}
              </div>
            </header>
            <div class="table">
              <table class="matches-table">
                <thead>
                  <tr>
                    <th>Machine</th>
                    <th>Path</th>
                    <th>Matches</th>
                    <th class="align-right">Count</th>
                    <th>Detected</th>
                  </tr>
                </thead>
                <tbody>
                  {#if matchesLoading}
                    <tr><td colspan="5" class="align-center text-light">Loading matches...</td></tr>
                  {:else if !selectedScan}
                    <tr><td colspan="5" class="align-center text-light">Select a scan</td></tr>
                  {:else}
                    {#each matches as match}
                      <tr>
                        <td>{match.hostname || match.machine_uuid}</td>
                        <td title={match.path}>{match.path}</td>
                        <td title={match.matches}>{match.matches}</td>
                        <td class="align-right">{match.count || 0}</td>
                        <td>{formatTimestamp(match.created_at)}</td>
                      </tr>
                    {:else}
                      <tr><td colspan="5" class="align-center text-light">No matches for selected scan</td></tr>
                    {/each}
                  {/if}
                </tbody>
              </table>
            </div>
            <footer class="hstack justify-between">
              <p class="text-light">
                Showing <strong>{matchStart}</strong> to <strong>{matchEnd}</strong> of <strong>{matchTotal}</strong> matches
              </p>
              <Pagination currentPage={matchPage} pageCount={matchPageCount} disabled={!selectedScan || matchesLoading} onPageChange={loadMatches} />
            </footer>
          </section>

          <section class="vstack gap-3">
            <h2>Targets</h2>
            <div class="table">
              <table class="targets-table">
                <thead>
                  <tr>
                    <th>Machine</th>
                    <th>Status</th>
                    <th>Dispatched</th>
                    <th>Completed</th>
                    <th>Error</th>
                  </tr>
                </thead>
                <tbody>
                  {#if targetsLoading}
                    <tr><td colspan="5" class="align-center text-light">Loading targets...</td></tr>
                  {:else if !selectedScan}
                    <tr><td colspan="5" class="align-center text-light">Select a scan</td></tr>
                  {:else}
                    {#each targets as target}
                      <tr>
                        <td>{target.hostname || target.machine_uuid}</td>
                        <td>{target.status}</td>
                        <td>{formatTimestamp(target.dispatched_at)}</td>
                        <td>{formatTimestamp(target.completed_at)}</td>
                        <td title={target.error || ''}>{target.error || ''}</td>
                      </tr>
                    {:else}
                      <tr><td colspan="5" class="align-center text-light">No targets for selected scan</td></tr>
                    {/each}
                  {/if}
                </tbody>
              </table>
            </div>
            <footer class="hstack justify-between">
              <p class="text-light"><strong>{targetTotal}</strong> targets</p>
            </footer>
          </section>
        </div>
      </div>

      <div role="tabpanel">
        <section class="vstack gap-3">
          <div class="hstack justify-between">
            <p class="text-light">{sources.length} source configs</p>
            <button type="button" onclick={openCreateSource}>Add Source Config</button>
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
                      <button type="button" class="cell-link" onclick={() => openEditSource(source)}>
                        {source.url}
                      </button>
                    </td>
                    <td>{source.enabled === false ? 'No' : 'Yes'}</td>
                    <td class="align-right">
                      <ActionsMenu label={`Actions for ${source.label || source.url}`}>
                        <button role="menuitem" type="button" onclick={() => openEditSource(source)}>Edit</button>
                        <hr />
                        <button role="menuitem" type="button" onclick={() => confirmDelete(source)}>Delete</button>
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
      <TextListInput
        label="Paths"
        bind:value={paths}
        placeholder="/tmp/%"
        addLabel="Add path"
        disabled={startingScan}
      />

      <SelectDropdown label="Group" options={groupOptions} bind:value={scanGroupID} disabled={startingScan} />

      <TextListInput
        label="Rule URLs"
        bind:value={ruleURLs}
        placeholder="https://rules.example.com/rule.yar"
        addLabel="Add rule URL"
        disabled={startingScan}
      />
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => scanDialog.close()}>Cancel</button>
      <button
        type="submit"
        disabled={startingScan || !hasPath || !hasRuleURL}
        aria-busy={startingScan ? 'true' : undefined}
      >
        {startingScan ? 'Starting...' : 'Start scan'}
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
        URL
        <input bind:value={sourceURL} placeholder="https://rules.example.com/.*" disabled={savingSource} />
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
      <button type="button" class="outline" onclick={() => sourceDialog.close()}>Cancel</button>
      <button type="submit" disabled={savingSource} aria-busy={savingSource ? 'true' : undefined}>
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
  .sources-table,
  .scans-table,
  .targets-table,
  .matches-table {
    table-layout: fixed;
    width: 100%;
  }
  .sources-table td,
  .scans-table td,
  .targets-table td,
  .matches-table td {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .scans-table tr[aria-current='true'] {
    background: var(--accent);
  }
</style>
