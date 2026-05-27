<script lang="ts">
  import {
    createGroup,
    fetchGroupMachines,
    fetchMachines,
    patchGroupMachines,
    updateGroup,
    type Group,
    type Machine
  } from '$lib/api'
  import { formatTimestamp, machineHostname } from '$lib/util'
  import ErrorMessage from './ErrorMessage.svelte'
  import MultiSelectDropdown from './MultiSelectDropdown.svelte'
  import Pagination from './Pagination.svelte'

  export let open = false
  export let group: Group | null = null
  export let onClose: () => void = () => {}
  export let onSaved: () => void = () => {}
  export let onChanged: () => void = () => {}

  const machinesPerPage = 100

  let dialog: HTMLDialogElement
  let preparedFor: string | null = null
  let title = ''
  let description = ''
  let error = ''
  let isSubmitting = false

  let machines: Machine[] = []
  let machinePage = 1
  let machinePageCount = 1
  let machineTotal = 0
  let machinesLoading = false
  let machinesError = ''

  let allMachines: Machine[] = []
  let addIds: string[] = []
  let membersSaving = false

  $: memberIds = machines.map((m) => m.uuid)
  $: addOptions = allMachines
    .filter((m) => !memberIds.includes(m.uuid))
    .map((m) => ({ value: m.uuid, label: machineHostname(m) }))

  $: if (open && dialog) {
    const key = group?.uuid || 'new'
    if (preparedFor !== key) {
      loadForm(group)
      preparedFor = key
      if (group?.uuid) {
        loadMachines(group.uuid, 1)
        loadAllMachines()
      }
    }
    if (!dialog.open) dialog.showModal()
  }

  $: if (!open && dialog) {
    preparedFor = null
    if (dialog.open) dialog.close()
  }

  function loadForm(record: Group | null) {
    title = record?.name || ''
    description = record?.description || ''
    error = ''
    isSubmitting = false
    machines = []
    machinePage = 1
    machinePageCount = 1
    machineTotal = 0
    machinesError = ''
    allMachines = []
    addIds = []
    membersSaving = false
  }

  async function loadMachines(uuid: string, targetPage: number) {
    machinesLoading = true
    machinesError = ''
    try {
      const data = await fetchGroupMachines(uuid, { page: targetPage, countPerPage: machinesPerPage })
      machines = data.machines || []
      machinePage = targetPage
      machinePageCount = data.page_count || 1
      machineTotal = data.total_count || machines.length
    } catch (err) {
      machinesError = (err as Error).message || 'Failed to fetch group hosts'
    } finally {
      machinesLoading = false
    }
  }

  async function loadAllMachines() {
    try {
      const data = await fetchMachines({ page: 1, countPerPage: 1000 })
      allMachines = data.machines || []
    } catch (err) {
      machinesError = (err as Error).message || 'Failed to load machines'
    }
  }

  function changeMachinePage(target: number) {
    if (group?.uuid && target > 0 && target <= machinePageCount) loadMachines(group.uuid, target)
  }

  async function patchMembers(changes: { add?: string[]; remove?: string[] }) {
    if (!group?.uuid) return
    membersSaving = true
    machinesError = ''
    try {
      await patchGroupMachines(group.uuid, changes)
      addIds = []
      await loadMachines(group.uuid, machinePage)
      onChanged()
    } catch (err) {
      machinesError = (err as Error).message || 'Failed to update group hosts'
    } finally {
      membersSaving = false
    }
  }

  function addHosts() {
    if (addIds.length === 0) return
    patchMembers({ add: addIds })
  }

  function removeHost(uuid: string) {
    patchMembers({ remove: [uuid] })
  }

  function handleClose() {
    onClose()
  }

  async function saveGroup(event: SubmitEvent) {
    event.preventDefault()
    isSubmitting = true
    error = ''
    try {
      const payload = { title, description }
      if (group?.uuid) await updateGroup(group.uuid, payload)
      else await createGroup(payload)
      onSaved()
      dialog.close()
    } catch (err) {
      error = (err as Error).message || 'Failed to save group'
    } finally {
      isSubmitting = false
    }
  }
</script>

<dialog bind:this={dialog} class:has-hosts={!!group?.uuid} onclose={handleClose} closedby="any">
  <form onsubmit={saveGroup}>
    <header>
      <h2>{group ? 'Edit Group' : 'Create Group'}</h2>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <div class="vstack">
      <label data-field>
        Name
        <input bind:value={title} required placeholder="Group name" />
      </label>

      <label data-field>
        Description
        <textarea bind:value={description} rows="3" placeholder="What this group represents"></textarea>
      </label>
    </div>

    {#if group?.uuid}
      <section class="hosts vstack gap-2">
        <h3 class="mb-0">Hosts <span class="text-light">({machineTotal})</span></h3>
        <ErrorMessage message={machinesError} onClose={() => (machinesError = '')} />

        <div class="vstack gap-2">
          <span>Add hosts</span>
          <div class="hstack gap-2 add-hosts">
            <div class="add-hosts-select">
              <MultiSelectDropdown
                label=""
                options={addOptions}
                bind:value={addIds}
                placeholder="Select hosts to add"
                searchPlaceholder="Search hosts..."
                emptyLabel="No more hosts to add"
                disabled={membersSaving || machinesLoading}
              />
            </div>
            <button
              type="button"
              class="add-hosts-button"
              onclick={addHosts}
              disabled={addIds.length === 0 || membersSaving}
              aria-busy={membersSaving ? 'true' : undefined}
            >
              Add{addIds.length ? ` (${addIds.length})` : ''}
            </button>
          </div>
        </div>

        <div class="hosts-scroll table" aria-busy={machinesLoading || membersSaving ? 'true' : undefined}>
          <table>
            <thead>
              <tr>
                <th>Host</th>
                <th>Platform</th>
                <th>OS</th>
                <th>Last seen</th>
                <th class="align-right"><span class="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              {#if machinesLoading}
                <tr><td colspan="5" class="align-center text-light">Loading hosts...</td></tr>
              {:else}
                {#each machines as machine}
                  <tr>
                    <td>
                      <a href="/machines/{machine.uuid}" class="cell-link">{machineHostname(machine)}</a>
                    </td>
                    <td>{machine.platform || 'Unknown'}</td>
                    <td>{machine.os_name || ''} {machine.os_version || ''}</td>
                    <td>{formatTimestamp(machine.last_seen_at)}</td>
                    <td class="align-right">
                      <button
                        type="button"
                        class="small outline"
                        data-variant="danger"
                        disabled={membersSaving}
                        onclick={() => removeHost(machine.uuid)}
                        aria-label={`Remove ${machineHostname(machine)} from group`}
                      >
                        Remove
                      </button>
                    </td>
                  </tr>
                {:else}
                  <tr><td colspan="5" class="align-center text-light">No hosts in this group</td></tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>
        {#if machinePageCount > 1}
          <Pagination
            currentPage={machinePage}
            pageCount={machinePageCount}
            disabled={machinesLoading}
            label="Group hosts pagination"
            onPageChange={changeMachinePage}
          />
        {/if}
      </section>
    {/if}

    <footer>
      <button type="button" class="outline" onclick={() => dialog.close()}>Cancel</button>
      <button type="submit" disabled={isSubmitting} aria-busy={isSubmitting ? 'true' : undefined}>
        {isSubmitting ? (group ? 'Updating...' : 'Creating...') : group ? 'Update' : 'Create'}
      </button>
    </footer>
  </form>
</dialog>

<style>
  .has-hosts {
    width: min(56rem, 92vw);
  }
  .hosts-scroll {
    max-height: 18rem;
    overflow-y: auto;
  }
  .add-hosts {
    align-items: flex-start;
  }
  .add-hosts-select {
    flex: 1;
    min-width: 0;
  }
  .add-hosts-button {
    min-width: 6rem;
  }
</style>
