<script lang="ts">
  import { fetchAlertRuleInstances, type AlertRule, type AlertInstance } from '$lib/api'
  import { formatTimestamp } from '$lib/util'
  import Pagination from './Pagination.svelte'
  import ErrorMessage from './ErrorMessage.svelte'

  let {
    open = $bindable(false),
    rule = null,
    onClose = () => {}
  }: {
    open?: boolean
    rule?: AlertRule | null
    onClose?: () => void
  } = $props()

  const countPerPage = 100
  const tabs: { value: string; label: string }[] = [
    { value: '', label: 'All' },
    { value: 'firing', label: 'Firing' },
    { value: 'pending', label: 'Pending' }
  ]

  let dialog = $state<HTMLDialogElement>()
  let preparedFor = $state<string | null>(null)
  let status = $state('')
  let instances = $state<AlertInstance[]>([])
  let page = $state(1)
  let pageCount = $state(1)
  let totalCount = $state(0)
  let loading = $state(false)
  let error = $state('')
  let expandedKey = $state<string | null>(null)

  $effect(() => {
    if (!open || !dialog) return
    const key = rule?.uuid || ''
    if (preparedFor !== key) {
      preparedFor = key
      load(1, '')
    }
    if (!dialog.open) dialog.showModal()
  })

  $effect(() => {
    if (open || !dialog) return
    preparedFor = null
    if (dialog.open) dialog.close()
  })

  async function load(targetPage = page, targetStatus = status) {
    if (!rule) return
    loading = true
    error = ''
    expandedKey = null
    try {
      const data = await fetchAlertRuleInstances(rule.uuid, {
        status: targetStatus,
        page: targetPage,
        countPerPage
      })
      instances = data.instances || []
      page = targetPage
      pageCount = data.page_count || 1
      totalCount = data.total_count || instances.length
      status = targetStatus
    } catch (err) {
      error = (err as Error).message || 'Failed to fetch alert instances'
    } finally {
      loading = false
    }
  }

  function selectTab(value: string) {
    if (value !== status) load(1, value)
  }

  function changePage(target: number) {
    if (target > 0 && target <= pageCount) load(target, status)
  }

  function statusVariant(value?: string): 'danger' | 'warning' {
    return value === 'firing' ? 'danger' : 'warning'
  }

  // Mirror the backend's alertHost(): prefer the hostname label, fall back to
  // the host id (see internal/alerts/smtp.go).
  function alertHost(labels?: Record<string, string>): string {
    const hostname = labels?.hostname?.trim()
    if (hostname) return hostname
    return labels?.host || ''
  }

  function toggleExpand(key: string) {
    expandedKey = expandedKey === key ? null : key
  }

  function pairs(obj?: Record<string, string>): [string, string][] {
    if (!obj) return []
    return Object.entries(obj).sort(([a], [b]) => a.localeCompare(b))
  }

  function handleClose() {
    if (open) {
      open = false
      onClose()
    }
  }
</script>

<dialog bind:this={dialog} class="alert-instances" closedby="any" onclose={handleClose}>
  <header>
    <h2>{rule?.name || 'Alert rule'} alerts</h2>
    <p>{totalCount} {totalCount === 1 ? 'alert' : 'alerts'}{status ? ` (${status})` : ''}</p>
  </header>

  <section class="vstack gap-3">
    <ErrorMessage message={error} onClose={() => (error = '')} />

    <div role="tablist">
      {#each tabs as tab}
        <button
          type="button"
          role="tab"
          aria-selected={status === tab.value}
          onclick={() => selectTab(tab.value)}
        >
          {tab.label}
        </button>
      {/each}
    </div>

    <div class="table" aria-busy={loading ? 'true' : undefined} data-spinner="overlay">
      <table>
        <thead>
          <tr>
            <th>Hostname</th>
            <th>Alert</th>
            <th>Status</th>
            <th>First seen</th>
            <th>Last seen</th>
          </tr>
        </thead>
        <tbody>
          {#if loading}
            <tr><td colspan="5" class="align-center text-light">Loading alerts...</td></tr>
          {:else}
            {#each instances as instance}
              <tr
                class="instance-row"
                aria-expanded={expandedKey === instance.alert_key}
                onclick={() => toggleExpand(instance.alert_key)}
              >
                <td>{alertHost(instance.labels) || '—'}</td>
                <td class="text-light">{instance.alert_key}</td>
                <td>
                  <span class="badge" data-variant={statusVariant(instance.status)}>
                    {instance.status}
                  </span>
                </td>
                <td>{formatTimestamp(instance.first_seen_at)}</td>
                <td>{formatTimestamp(instance.last_seen_at)}</td>
              </tr>
              {#if expandedKey === instance.alert_key}
                <tr class="instance-detail">
                  <td colspan="5">
                    <div class="detail-grid">
                      {#if instance.annotations && pairs(instance.annotations).length}
                        <section>
                          <h3>Annotations</h3>
                          <dl>
                            {#each pairs(instance.annotations) as [k, v]}
                              <dt>{k}</dt>
                              <dd>{v}</dd>
                            {/each}
                          </dl>
                        </section>
                      {/if}
                      <section>
                        <h3>Labels</h3>
                        {#if pairs(instance.labels).length}
                          <dl>
                            {#each pairs(instance.labels) as [k, v]}
                              <dt>{k}</dt>
                              <dd>{v}</dd>
                            {/each}
                          </dl>
                        {:else}
                          <p class="text-light">No labels</p>
                        {/if}
                      </section>
                      <section>
                        <h3>Timing</h3>
                        <dl>
                          <dt>Status</dt>
                          <dd>{instance.status}</dd>
                          <dt>First seen</dt>
                          <dd>{formatTimestamp(instance.first_seen_at)}</dd>
                          <dt>Last seen</dt>
                          <dd>{formatTimestamp(instance.last_seen_at)}</dd>
                          {#if instance.last_notified_at}
                            <dt>Last notified</dt>
                            <dd>{formatTimestamp(instance.last_notified_at)}</dd>
                          {/if}
                        </dl>
                      </section>
                    </div>
                  </td>
                </tr>
              {/if}
            {:else}
              <tr><td colspan="5" class="align-center text-light">No alerts for this filter</td></tr>
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
      label="Alert instances pagination"
      onPageChange={changePage}
    />
    <button type="button" class="outline" onclick={() => dialog?.close()}>Close</button>
  </footer>
</dialog>

<style>
  .alert-instances {
    width: min(64rem, 92vw);
  }
  .alert-instances > section {
    max-height: 60vh;
  }
  .instance-row {
    cursor: pointer;
  }
  .instance-detail > td {
    background: var(--oat-color-bg-subtle, rgba(0, 0, 0, 0.03));
  }
  .detail-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
    gap: 1rem 2rem;
    padding: 0.5rem 0.25rem;
  }
  .detail-grid h3 {
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    margin: 0 0 0.5rem;
    opacity: 0.7;
  }
  .detail-grid dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 0.25rem 0.75rem;
    margin: 0;
  }
  .detail-grid dt {
    font-weight: 600;
    opacity: 0.8;
    word-break: break-word;
  }
  .detail-grid dd {
    margin: 0;
    word-break: break-word;
  }
</style>
