<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import { fetchDashboardOverview, type DashboardOverview } from '$lib/api'
  import { canFrom, me } from '$lib/auth'
  import { formatTimestamp } from '$lib/util'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import CircleHelp from '@lucide/svelte/icons/circle-help'

  let data = $state<DashboardOverview | null>(null)
  let loading = $state(true)
  let error = $state('')
  let now = $state(Date.now())

  const hasAccess = $derived(canFrom($me, 'dashboard', 'view'))

  const severityVariant: Record<string, string> = {
    critical: 'danger',
    high: 'danger',
    medium: 'warning',
    low: 'info',
    info: 'info'
  }

  const updatedAgo = $derived.by(() => {
    if (!data?.generated_at) return ''
    const secs = Math.max(0, Math.round((now - new Date(data.generated_at).getTime()) / 1000))
    if (secs < 60) return `updated ${secs}s ago`
    const mins = Math.round(secs / 60)
    return `updated ${mins}m ago`
  })

  let poll: ReturnType<typeof setInterval> | undefined
  let tick: ReturnType<typeof setInterval> | undefined
  let triggered = $state(false)

  // `me` is populated by the layout after mount, so gate the first load reactively
  // rather than reading hasAccess once in onMount (which races the store).
  $effect(() => {
    if (hasAccess && !triggered) {
      triggered = true
      void load()
    } else if ($me && !hasAccess) {
      loading = false
    }
  })

  onMount(() => {
    tick = setInterval(() => (now = Date.now()), 1000)
    poll = setInterval(() => {
      if (hasAccess && document.visibilityState === 'visible') void load()
    }, 30000)
  })

  onDestroy(() => {
    clearInterval(poll)
    clearInterval(tick)
  })

  async function load() {
    loading = true
    error = ''
    try {
      data = await fetchDashboardOverview({ top: 5 })
      now = Date.now()
    } catch (err) {
      error = (err as Error).message || 'Failed to load dashboard'
    } finally {
      loading = false
    }
  }

  function pct(value: number, max: number): number {
    if (max <= 0) return 0
    return Math.round((value / max) * 100)
  }

  const maxPlatformTotal = $derived(
    Math.max(1, ...(data?.machines.by_platform.map((p) => p.total) ?? [0]))
  )
  const maxFailing = $derived(
    Math.max(1, ...(data?.compliance.top_failing_policies.map((p) => p.failing_count) ?? [0]))
  )

  const postureHelp =
    'Share of all policy checks across every enrolled machine, by their latest result: ' +
    'pass, fail, or unknown (not yet reported).'

  const scoreHelp =
    'Compliance score = round(100 × passing checks ÷ total checks) for each machine. ' +
    '100 means all checks pass, 0 means all fail. Machines are ranked by their passing ratio.'
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">Dashboard</h1>
      <p class="text-light">Fleet health, compliance, and security at a glance</p>
    </div>
    {#if data}
      <div class="hstack gap-2 items-center">
        <span class="text-light updated-caption">{updatedAgo}</span>
        <button type="button" class="outline" onclick={() => void load()} aria-busy={loading ? 'true' : undefined}>
          <RefreshCw size={16} aria-hidden="true" /> Refresh
        </button>
      </div>
    {/if}
  </header>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if $me && !hasAccess}
    <article class="card vstack gap-2 align-center empty-access">
      <p>You don't have access to the dashboard.</p>
      <a href="/inventory">Go to inventory</a>
    </article>
  {:else if loading && !data}
    <Spinner fill />
  {:else if data}
    <!-- Counter strip -->
    <div class="stat-strip">
      <article class="card stat">
        <span class="stat-value">{data.machines.total}</span>
        <span class="text-light">Machines</span>
        {#if data.machines.never_reported > 0}
          <span class="text-light stat-sub">{data.machines.never_reported} never reported</span>
        {/if}
      </article>
      <article class="card stat">
        <span class="stat-value text-success">{data.machines.online}</span>
        <span class="text-light">Online</span>
      </article>
      <article class="card stat">
        <span class="stat-value text-danger">{data.machines.offline}</span>
        <span class="text-light">Offline</span>
      </article>
      <article class="card stat">
        <span class="stat-value">{data.compliance.score ?? '—'}{#if data.compliance.score !== null}<small>/100</small>{/if}</span>
        <span class="text-light">Compliance score</span>
      </article>
      <article class="card stat">
        <span class="stat-value">{data.security.firing_alerts.total}</span>
        <span class="text-light">Active alerts</span>
      </article>
    </div>

    <div class="row">
      <!-- Compliance posture -->
      <div class="col-6">
        <article class="card vstack gap-2 panel">
          <div class="panel-head">
            <h3>Compliance posture</h3>
            <button
              type="button"
              class="help-btn"
              title={postureHelp}
              data-tooltip-placement="left"
              aria-label={postureHelp}
            >
              <CircleHelp size={16} aria-hidden="true" />
            </button>
          </div>
          {#if data.compliance.policy_rows.passing + data.compliance.policy_rows.failing + data.compliance.policy_rows.unknown === 0}
            <p class="text-light">No policies configured</p>
          {:else}
            <div class="bar stacked">
              <span class="seg success" style="width:{pct(data.compliance.policy_rows.passing, data.compliance.policy_rows.passing + data.compliance.policy_rows.failing + data.compliance.policy_rows.unknown)}%"></span>
              <span class="seg danger" style="width:{pct(data.compliance.policy_rows.failing, data.compliance.policy_rows.passing + data.compliance.policy_rows.failing + data.compliance.policy_rows.unknown)}%"></span>
              <span class="seg warning" style="width:{pct(data.compliance.policy_rows.unknown, data.compliance.policy_rows.passing + data.compliance.policy_rows.failing + data.compliance.policy_rows.unknown)}%"></span>
            </div>
            <div class="hstack gap-4 legend text-light">
              <span><span class="dot success"></span> {data.compliance.policy_rows.passing} checks pass</span>
              <span><span class="dot danger"></span> {data.compliance.policy_rows.failing} checks fail</span>
              <span><span class="dot warning"></span> {data.compliance.policy_rows.unknown} checks unknown</span>
            </div>
          {/if}
        </article>
      </div>

      <!-- Top failing policies -->
      <div class="col-6">
        <article class="card vstack gap-2 panel">
          <h3>Top failing policies</h3>
          {#if data.compliance.top_failing_policies.length === 0}
            <p class="text-light">No failing policies</p>
          {:else}
            <div class="vstack gap-2">
              {#each data.compliance.top_failing_policies as p}
                <div class="ranked">
                  <a href="/policies" class="ranked-label" title={p.name}>{p.name}</a>
                  <div class="bar"><span class="seg danger" style="width:{pct(p.failing_count, maxFailing)}%"></span></div>
                  <span class="ranked-count">{p.failing_count}</span>
                </div>
              {/each}
            </div>
          {/if}
        </article>
      </div>

      <!-- Most compliant -->
      <div class="col-6">
        <article class="card vstack gap-2 panel">
          <div class="panel-head">
            <h3>Most compliant machines</h3>
            <button
              type="button"
              class="help-btn"
              title={scoreHelp}
              data-tooltip-placement="left"
              aria-label={scoreHelp}
            >
              <CircleHelp size={16} aria-hidden="true" />
            </button>
          </div>
          {#if data.compliance.most_compliant.length === 0}
            <p class="text-light">No machines with policies assigned</p>
          {:else}
            <div class="table">
              <table>
                <thead>
                  <tr><th>Machine</th><th class="align-right">Score</th><th class="align-right">Passing</th></tr>
                </thead>
                <tbody>
                  {#each data.compliance.most_compliant as n}
                    <tr>
                      <td><a href="/machines/{n.uuid}">{n.display_name || n.hostname}</a></td>
                      <td class="align-right">{n.score}</td>
                      <td class="align-right">{n.passing}/{n.total}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </article>
      </div>

      <!-- Least compliant -->
      <div class="col-6">
        <article class="card vstack gap-2 panel">
          <div class="panel-head">
            <h3>Least compliant machines</h3>
            <button
              type="button"
              class="help-btn"
              title={scoreHelp}
              data-tooltip-placement="left"
              aria-label={scoreHelp}
            >
              <CircleHelp size={16} aria-hidden="true" />
            </button>
          </div>
          {#if data.compliance.least_compliant.length === 0}
            <p class="text-light">No machines with policies assigned</p>
          {:else}
            <div class="table">
              <table>
                <thead>
                  <tr><th>Machine</th><th class="align-right">Score</th><th class="align-right">Passing</th></tr>
                </thead>
                <tbody>
                  {#each data.compliance.least_compliant as n}
                    <tr>
                      <td><a href="/machines/{n.uuid}">{n.display_name || n.hostname}</a></td>
                      <td class="align-right">{n.score}</td>
                      <td class="align-right">{n.passing}/{n.total}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </article>
      </div>

      <!-- Platform breakdown -->
      <div class="col-6">
        <article class="card vstack gap-2 panel">
          <h3>Platform breakdown</h3>
          {#if data.machines.by_platform.length === 0}
            <p class="text-light">No machines enrolled yet</p>
          {:else}
            <div class="vstack gap-2">
              {#each data.machines.by_platform as p}
                <div class="ranked">
                  <span class="ranked-label">{p.platform || 'unknown'}</span>
                  <div class="bar"><span class="seg primary" style="width:{pct(p.total, maxPlatformTotal)}%"></span></div>
                  <span class="ranked-count">{p.total}</span>
                </div>
              {/each}
            </div>
          {/if}
        </article>
      </div>

      <!-- Active alerts -->
      <div class="col-6">
        <article class="card vstack gap-2 panel">
          <h3>Active alerts</h3>
          {#if data.security.firing_alert_list.length === 0}
            <p class="text-light">No firing alerts</p>
          {:else}
            <ul class="plain">
              {#each data.security.firing_alert_list as a}
                <li class="hstack justify-between gap-2">
                  <a href="/alerts" class="hstack gap-2 items-center">
                    <span class="badge" data-variant={severityVariant[a.severity]}>{a.severity}</span>
                    <span>{a.name}</span>
                  </a>
                  <span class="text-light">{formatTimestamp(a.last_seen_at)}</span>
                </li>
              {/each}
            </ul>
          {/if}
        </article>
      </div>

      <!-- Recent YARA matches -->
      <div class="col-6">
        <article class="card vstack gap-2 panel">
          <h3>Recent YARA matches</h3>
          {#if data.security.recent_yara_matches.length === 0}
            <p class="text-light">No recent matches</p>
          {:else}
            <ul class="plain">
              {#each data.security.recent_yara_matches as m}
                <li class="hstack justify-between gap-2">
                  <a href="/yara" title={m.path}>{m.rules} on {m.hostname}</a>
                  <span class="text-light">{formatTimestamp(m.matched_at)}</span>
                </li>
              {/each}
            </ul>
          {/if}
        </article>
      </div>

      <!-- Recently enrolled -->
      <div class="col-6">
        <article class="card vstack gap-2 panel">
          <h3>Recently enrolled</h3>
          {#if data.recently_enrolled.length === 0}
            <p class="text-light">No machines enrolled yet</p>
          {:else}
            <ul class="plain">
              {#each data.recently_enrolled as m}
                <li class="hstack justify-between gap-2">
                  <a href="/machines/{m.uuid}">{m.display_name || m.hostname}</a>
                  <span class="text-light">{formatTimestamp(m.enrolled_at)}</span>
                </li>
              {/each}
            </ul>
          {/if}
        </article>
      </div>
    </div>
  {/if}
</section>

<style>
  .stat-strip {
    display: flex;
    flex-wrap: wrap;
    gap: 1rem;
  }
  .stat {
    flex: 1 1 8rem;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .stat-value {
    font-size: 1.75rem;
    font-weight: 600;
    line-height: 1.1;
  }
  .stat-value small {
    font-size: 0.9rem;
    font-weight: 400;
    color: var(--muted-foreground);
  }
  .stat-sub,
  .updated-caption {
    font-size: 0.8rem;
  }
  .panel h3 {
    margin: 0;
  }
  .panel-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
  }
  .help-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.5rem;
    height: 1.5rem;
    padding: 0;
    border: none;
    border-radius: 50%;
    background: transparent;
    color: var(--muted-foreground);
    cursor: pointer;
  }
  .help-btn:hover {
    color: var(--foreground);
    background: var(--muted);
  }
  /* oat copies title -> data-tooltip at runtime and renders it nowrap; let the
     longer compliance explanations wrap instead of clipping near the sidebar. */
  :global(.help-btn[data-tooltip]::after) {
    white-space: normal;
    width: max-content;
    max-width: 18rem;
    text-align: left;
    line-height: 1.4;
  }
  .bar {
    display: flex;
    width: 100%;
    height: 0.6rem;
    border-radius: 999px;
    overflow: hidden;
    background: var(--muted);
  }
  .bar.stacked {
    height: 0.9rem;
  }
  .seg {
    display: block;
    height: 100%;
  }
  .seg.success {
    background: var(--success);
  }
  .seg.danger {
    background: var(--danger);
  }
  .seg.warning {
    background: var(--warning);
  }
  .seg.primary {
    background: var(--primary);
  }
  .legend {
    font-size: 0.85rem;
    flex-wrap: wrap;
  }
  .dot {
    display: inline-block;
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 50%;
    vertical-align: middle;
  }
  .dot.success {
    background: var(--success);
  }
  .dot.danger {
    background: var(--danger);
  }
  .dot.warning {
    background: var(--warning);
  }
  .ranked {
    display: grid;
    grid-template-columns: 9rem 1fr auto;
    align-items: center;
    gap: 0.75rem;
  }
  .ranked-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ranked-count {
    font-variant-numeric: tabular-nums;
    min-width: 2rem;
    text-align: right;
  }
  ul.plain {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }
  .empty-access {
    padding: 2rem;
  }
</style>
