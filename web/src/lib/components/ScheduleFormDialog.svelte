<script lang="ts">
  import {
    createSchedule,
    fetchAllQueries,
    fetchGroups,
    updateSchedule,
    type Group,
    type Query,
    type Schedule
  } from '$lib/api'
  import ErrorMessage from './ErrorMessage.svelte'
  import MultiSelectDropdown from './MultiSelectDropdown.svelte'

  export let open = false
  export let schedule: Schedule | null = null
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}

  let dialog: HTMLDialogElement
  let preparedFor: string | null = null
  let queries: Query[] = []
  let availableGroups: Group[] = []
  let selectedQuery = ''
  let title = ''
  let interval = 3600
  let platform = 'all'
  let snapshot = false
  let groupIds: string[] = []
  let error = ''
  let isSubmitting = false

  $: if (open && dialog) {
    const key = schedule?.uuid || 'new'
    if (preparedFor !== key) {
      loadForm(schedule)
      loadQueries()
      loadGroups()
      preparedFor = key
    }
    if (!dialog.open) dialog.showModal()
  }

  $: if (!open && dialog) {
    preparedFor = null
    if (dialog.open) dialog.close()
  }

  function loadForm(record: Schedule | null) {
    selectedQuery = record?.query?.uuid || ''
    title = record?.title || ''
    interval = record?.interval || 3600
    platform = record?.platform || 'all'
    snapshot = Boolean(record?.snapshot)
    groupIds = (record?.groups || []).map((g) => g.uuid)
    error = ''
    isSubmitting = false
  }

  async function loadQueries() {
    try {
      const data = await fetchAllQueries()
      queries = data.queries || []
    } catch (err) {
      error = (err as Error).message || 'Failed to fetch queries'
    }
  }

  async function loadGroups() {
    try {
      const data = await fetchGroups({ page: 1, countPerPage: 1000 })
      availableGroups = data.groups || []
    } catch (err) {
      error = (err as Error).message || 'Failed to load groups'
    }
  }

  function handleClose() {
    onClose()
  }

  async function saveSchedule(event: SubmitEvent) {
    event.preventDefault()
    isSubmitting = true
    error = ''
    try {
      const payload = {
        query_id: selectedQuery,
        title,
        interval: Number.parseInt(String(interval), 10),
        platform,
        snapshot,
        group_ids: groupIds
      }
      if (schedule?.uuid) await updateSchedule(schedule.uuid, payload)
      else await createSchedule(payload)
      onSaved()
      dialog.close()
    } catch (err) {
      error = (err as Error).message || 'Failed to save schedule'
    } finally {
      isSubmitting = false
    }
  }
</script>

<dialog bind:this={dialog} onclose={handleClose} closedby="any">
  <form onsubmit={saveSchedule}>
    <header>
      <h2>{schedule ? 'Edit Schedule' : 'Create Schedule'}</h2>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <div class="vstack">
      <label data-field>
        Query
        <select bind:value={selectedQuery} required>
          <option value="">Select a query...</option>
          {#each queries as query}
            <option value={query.uuid}>{query.title || query.description || query.query}</option>
          {/each}
        </select>
      </label>

      <label data-field>
        Title
        <input bind:value={title} required placeholder="Schedule title" />
      </label>

      <label data-field>
        Interval (seconds)
        <input type="number" bind:value={interval} required min="1" max="604800" />
      </label>

      <label data-field>
        Platform
        <select bind:value={platform}>
          <option value="all">All</option>
          <option value="any">Any</option>
          <option value="posix">POSIX</option>
          <option value="darwin">macOS</option>
          <option value="linux">Linux</option>
          <option value="windows">Windows</option>
        </select>
      </label>

      <div data-field class="vstack gap-2">
        <span>Targets</span>
        <small data-hint
          >{groupIds.length === 0 ? 'All machines for this platform' : `${groupIds.length} groups selected`}</small
        >
        <MultiSelectDropdown
          label="Groups"
          options={availableGroups}
          bind:value={groupIds}
          placeholder="All machines for this platform"
          searchPlaceholder="Search groups..."
          emptyLabel="No groups available yet"
        />
      </div>

      <label>
        <input type="checkbox" bind:checked={snapshot} />
        Snapshot
      </label>
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => dialog.close()}>Cancel</button>
      <button type="submit" disabled={isSubmitting} aria-busy={isSubmitting ? 'true' : undefined}>
        {isSubmitting
          ? schedule
            ? 'Updating...'
            : 'Creating...'
          : schedule
            ? 'Update'
            : 'Create'}
      </button>
    </footer>
  </form>
</dialog>
