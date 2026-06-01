<script lang="ts">
  import { onMount } from 'svelte'
  import { deleteAlertRule, fetchAlertRules, type AlertRule } from '$lib/api'
  import { formatTimestamp } from '$lib/util'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'
  import ActionsMenu from '$lib/components/ActionsMenu.svelte'
  import AlertRuleFormDialog from '$lib/components/AlertRuleFormDialog.svelte'
  import { canFrom, me } from '$lib/auth'

  let rules = $state<AlertRule[]>([])
  let currentPage = $state(1)
  let pageCount = $state(1)
  let totalCount = $state(0)
  const countPerPage = 10
  let error = $state('')
  let loading = $state(true)
  let formOpen = $state(false)
  let editing = $state<AlertRule | null>(null)
  let deleteOpen = $state(false)
  let selected = $state<AlertRule | null>(null)
  let isDeleting = $state(false)

  const canCreate = $derived(canFrom($me, 'alert_rule', 'create'))
  const canUpdate = $derived(canFrom($me, 'alert_rule', 'update'))
  const canDelete = $derived(canFrom($me, 'alert_rule', 'delete'))

  const severityVariant: Record<string, string> = {
    critical: 'danger',
    high: 'danger',
    medium: 'warning',
    low: 'info',
    info: 'info'
  }

  onMount(load)

  async function load() {
    loading = true
    error = ''
    try {
      const data = await fetchAlertRules({ page: currentPage, countPerPage })
      rules = data.rules || []
      pageCount = data.page_count || 1
      totalCount = data.total_count || rules.length
    } catch (err) {
      error = (err as Error).message || 'Failed to fetch alert rules'
    } finally {
      loading = false
    }
  }

  async function changePage(page: number) {
    if (page > 0 && page <= pageCount) {
      currentPage = page
      await load()
    }
  }

  function openCreate() {
    if (!canCreate) return
    editing = null
    formOpen = true
  }

  function openEdit(rule: AlertRule) {
    if (!canUpdate) return
    editing = rule
    formOpen = true
  }

  async function handleSaved() {
    formOpen = false
    await load()
  }

  function confirmDelete(rule: AlertRule) {
    if (!canDelete) return
    selected = rule
    deleteOpen = true
  }

  async function remove() {
    if (!selected) return
    isDeleting = true
    error = ''
    try {
      await deleteAlertRule(selected.uuid)
      deleteOpen = false
      selected = null
      await load()
    } catch (err) {
      error = (err as Error).message || 'Failed to delete alert rule'
    } finally {
      isDeleting = false
    }
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">Alerts</h1>
      <p class="text-light">Rules that watch the fleet and notify configured targets</p>
    </div>
    {#if canCreate}
      <button type="button" onclick={openCreate}>Create Rule</button>
    {/if}
  </header>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if loading}
    <Spinner />
  {:else}
    <div class="table">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Source</th>
            <th>Severity</th>
            <th>Status</th>
            <th>Last evaluated</th>
            <th class="align-right"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {#each rules as rule}
            <tr>
              <td>
                <button type="button" class="cell-link" onclick={() => openEdit(rule)}>
                  {rule.name || 'Untitled'}
                </button>
                {#if rule.description}<p class="text-light">{rule.description}</p>{/if}
              </td>
              <td><span class="badge outline">{rule.source}</span></td>
              <td>
                <span class="badge" data-variant={severityVariant[rule.severity || 'medium']}>
                  {rule.severity}
                </span>
              </td>
              <td>
                <span class="badge" data-variant={rule.enabled ? 'success' : 'warning'}>
                  {rule.enabled ? 'Enabled' : 'Disabled'}
                </span>
              </td>
              <td>{formatTimestamp(rule.last_evaluated_at)}</td>
              <td class="align-right">
                {#if canUpdate || canDelete}
                  <ActionsMenu label={`Actions for ${rule.name || 'rule'}`}>
                    {#if canUpdate}
                      <button role="menuitem" type="button" onclick={() => openEdit(rule)}>Edit</button>
                    {/if}
                    {#if canUpdate && canDelete}<hr />{/if}
                    {#if canDelete}
                      <button role="menuitem" type="button" onclick={() => confirmDelete(rule)}>Delete</button>
                    {/if}
                  </ActionsMenu>
                {/if}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="6" class="align-center text-light">No alert rules found</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <footer class="hstack justify-between">
      <p class="text-light"><strong>{totalCount}</strong> rules</p>
      <Pagination {currentPage} {pageCount} onPageChange={changePage} />
    </footer>
  {/if}
</section>

<AlertRuleFormDialog
  open={formOpen}
  rule={editing}
  onClose={() => (formOpen = false)}
  onSaved={handleSaved}
/>

<ConfirmDialog
  bind:open={deleteOpen}
  title="Delete Alert Rule"
  message="Are you sure you want to delete this alert rule? This action cannot be undone."
  confirming={isDeleting}
  confirmingLabel="Deleting..."
  onConfirm={remove}
  onCancel={() => (selected = null)}
/>
