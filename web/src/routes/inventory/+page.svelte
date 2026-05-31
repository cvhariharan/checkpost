<script lang="ts">
  import { onMount } from 'svelte'
  import {
    createOwner,
    deleteOwner,
    fetchMachines,
    fetchOwners,
    updateMachineInventory,
    updateOwner,
    type DeviceOwner,
    type Machine
  } from '$lib/api'
  import { formatTimestamp, isOnline, machineHostname } from '$lib/util'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import SearchInput from '$lib/components/SearchInput.svelte'
  import SelectFilter from '$lib/components/SelectFilter.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'
  import ActionsMenu from '$lib/components/ActionsMenu.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import OsqueryBootstrapDialog from '$lib/components/OsqueryBootstrapDialog.svelte'
  import { canFrom, me } from '$lib/auth'

  type OatTabsElement = HTMLElement & { activeIndex: number }
  type FilterOption = { value: string; label: string }

  const machineCountPerPage = 10
  const ownerCountPerPage = 10

  let machines = $state<Machine[]>([])
  let owners = $state<DeviceOwner[]>([])
  let ownerOptions = $state<DeviceOwner[]>([])
  let searchTerm = $state('')
  let selectedPlatform = $state('')
  let selectedOwner = $state('')
  let assignmentFilter = $state('')
  let ownerSearchTerm = $state('')
  let activeTabIndex = $state(0)
  let error = $state('')
  let ready = $state(false)
  let loadingMachines = $state(true)
  let loadingOwners = $state(true)
  let loadingOwnerOptions = $state(true)
  let currentMachinePage = $state(1)
  let machinePageCount = $state(1)
  let machineTotalCount = $state(0)
  let currentOwnerPage = $state(1)
  let ownerPageCount = $state(1)
  let ownerTotalCount = $state(0)
  let initialized = $state(false)
  let previousMachineFilterKey = $state('')
  let previousMachineSearch = $state('')
  let previousOwnerSearch = $state('')
  let machineSearchTimer: ReturnType<typeof setTimeout> | undefined
  let ownerSearchTimer: ReturnType<typeof setTimeout> | undefined

  let tabs = $state<OatTabsElement>()
  let inventoryDialog = $state<HTMLDialogElement>()
  let ownerDialog = $state<HTMLDialogElement>()
  let bootstrapDialogOpen = $state(false)
  let editingMachine = $state<Machine | null>(null)
  let editingOwner = $state<DeviceOwner | null>(null)
  let ownerToDelete = $state<DeviceOwner | null>(null)
  let savingInventory = $state(false)
  let savingOwner = $state(false)
  let deletingOwner = $state(false)

  let inventoryOwnerID = $state('')
  let internalTrackingID = $state('')
  let inventoryNotes = $state('')

  let ownerDisplayName = $state('')
  let ownerEmail = $state('')
  let ownerExternalID = $state('')
  let ownerDepartment = $state('')
  let ownerTitle = $state('')
  let ownerPhone = $state('')
  let ownerNotes = $state('')

  const allPlatforms = $derived(Array.from(
    new Set(machines.map((m) => m.platform).filter((p): p is string => Boolean(p)))
  ).sort())
  const ownerFilterOptions = $derived(ownerOptions.map<FilterOption>((owner) => ({
    value: owner.uuid,
    label: owner.display_name || owner.email || owner.uuid
  })))
  const machineStartResult = $derived(machines.length === 0 ? 0 : (currentMachinePage - 1) * machineCountPerPage + 1)
  const machineEndResult = $derived(Math.min(currentMachinePage * machineCountPerPage, machineTotalCount))
  const ownerStartResult = $derived(owners.length === 0 ? 0 : (currentOwnerPage - 1) * ownerCountPerPage + 1)
  const ownerEndResult = $derived(Math.min(currentOwnerPage * ownerCountPerPage, ownerTotalCount))
  const canCreateInventory = $derived(canFrom($me, 'inventory', 'create'))
  const canUpdateInventory = $derived(canFrom($me, 'inventory', 'update'))
  const canDeleteInventory = $derived(canFrom($me, 'inventory', 'delete'))
  const canViewSettings = $derived(canFrom($me, 'setting', 'view'))

  onMount(() => {
    void initialize()

    return () => {
      clearTimeout(machineSearchTimer)
      clearTimeout(ownerSearchTimer)
    }
  })

  $effect(() => {
    if (!tabs || tabs.activeIndex === activeTabIndex) return
    tabs.activeIndex = activeTabIndex
  })

  $effect(() => {
    if (!initialized) return
    const nextFilterKey = JSON.stringify([selectedPlatform, selectedOwner, assignmentFilter])
    if (nextFilterKey !== previousMachineFilterKey) {
      previousMachineFilterKey = nextFilterKey
      currentMachinePage = 1
      void loadMachines()
    }
  })

  $effect(() => {
    if (!initialized || searchTerm === previousMachineSearch) return
    previousMachineSearch = searchTerm
    currentMachinePage = 1
    clearTimeout(machineSearchTimer)
    machineSearchTimer = setTimeout(() => {
      void loadMachines()
    }, 250)
  })

  $effect(() => {
    if (!initialized || ownerSearchTerm === previousOwnerSearch) return
    previousOwnerSearch = ownerSearchTerm
    currentOwnerPage = 1
    clearTimeout(ownerSearchTimer)
    ownerSearchTimer = setTimeout(() => {
      void loadOwners()
    }, 250)
  })

  function handleTabChange(event: CustomEvent<{ index: number }>) {
    activeTabIndex = event.detail.index
  }

  async function initialize() {
    error = ''
    try {
      await Promise.all([loadOwnerOptions(), loadMachines(), loadOwners()])
    } catch (err) {
      error = (err as Error).message || 'Failed to load inventory'
    } finally {
      previousMachineFilterKey = JSON.stringify([selectedPlatform, selectedOwner, assignmentFilter])
      previousMachineSearch = searchTerm
      previousOwnerSearch = ownerSearchTerm
      initialized = true
      ready = true
    }
  }

  async function loadOwnerOptions() {
    loadingOwnerOptions = true
    try {
      const data = await fetchOwners({ page: 1, countPerPage: 1000 })
      ownerOptions = data.owners || []
      if (selectedOwner && !ownerOptions.some((owner) => owner.uuid === selectedOwner)) {
        selectedOwner = ''
      }
    } catch (err) {
      error = (err as Error).message || 'Failed to load owner options'
    } finally {
      loadingOwnerOptions = false
    }
  }

  async function loadMachines() {
    loadingMachines = true
    error = ''
    try {
      const data = await fetchMachines({
        page: currentMachinePage,
        countPerPage: machineCountPerPage,
        query: searchTerm,
        platform: selectedPlatform,
        ownerID: selectedOwner,
        assigned: assignmentFilter
      })
      machines = data.machines || []
      machinePageCount = Math.max(1, data.page_count || 1)
      machineTotalCount = data.total_count || 0
      if (currentMachinePage > machinePageCount) {
        currentMachinePage = machinePageCount
        await loadMachines()
      }
    } catch (err) {
      machines = []
      machinePageCount = 1
      machineTotalCount = 0
      error = (err as Error).message || 'Failed to load devices'
    } finally {
      loadingMachines = false
    }
  }

  async function loadOwners() {
    loadingOwners = true
    error = ''
    try {
      const data = await fetchOwners({
        page: currentOwnerPage,
        countPerPage: ownerCountPerPage,
        query: ownerSearchTerm
      })
      owners = data.owners || []
      ownerPageCount = Math.max(1, data.page_count || 1)
      ownerTotalCount = data.total_count || 0
      if (currentOwnerPage > ownerPageCount) {
        currentOwnerPage = ownerPageCount
        await loadOwners()
      }
    } catch (err) {
      owners = []
      ownerPageCount = 1
      ownerTotalCount = 0
      error = (err as Error).message || 'Failed to load owners'
    } finally {
      loadingOwners = false
    }
  }

  function statusLabel(machine: Machine): string {
    if (!machine.last_seen_at && !machine.enrolled_at) return 'unknown'
    return isOnline(machine) ? 'active' : 'offline'
  }

  function statusVariant(machine: Machine): 'success' | 'danger' | 'warning' {
    const label = statusLabel(machine)
    if (label === 'active') return 'success'
    if (label === 'offline') return 'danger'
    return 'warning'
  }

  function openInventoryDialog(machine: Machine) {
    if (!canUpdateInventory) return
    editingMachine = machine
    inventoryOwnerID = machine.inventory?.owner?.uuid || ''
    internalTrackingID = machine.inventory?.internal_tracking_id || ''
    inventoryNotes = machine.inventory?.notes || ''
    inventoryDialog?.showModal()
  }

  async function saveInventory(event: SubmitEvent) {
    event.preventDefault()
    if (!editingMachine?.uuid) return
    savingInventory = true
    error = ''
    try {
      await updateMachineInventory(editingMachine.uuid, {
        owner_id: inventoryOwnerID || null,
        internal_tracking_id: internalTrackingID,
        notes: inventoryNotes
      })
      inventoryDialog?.close()
      await Promise.all([loadMachines(), loadOwners()])
    } catch (err) {
      error = (err as Error).message || 'Failed to save machine inventory'
    } finally {
      savingInventory = false
    }
  }

  function openCreateOwner() {
    if (!canCreateInventory) return
    editingOwner = null
    ownerDisplayName = ''
    ownerEmail = ''
    ownerExternalID = ''
    ownerDepartment = ''
    ownerTitle = ''
    ownerPhone = ''
    ownerNotes = ''
    ownerDialog?.showModal()
  }

  function openEditOwner(owner: DeviceOwner) {
    if (!canUpdateInventory) return
    editingOwner = owner
    ownerDisplayName = owner.display_name || ''
    ownerEmail = owner.email || ''
    ownerExternalID = owner.external_id || ''
    ownerDepartment = owner.department || ''
    ownerTitle = owner.title || ''
    ownerPhone = owner.phone || ''
    ownerNotes = owner.notes || ''
    ownerDialog?.showModal()
  }

  async function saveOwner(event: SubmitEvent) {
    event.preventDefault()
    savingOwner = true
    error = ''
    const payload = {
      display_name: ownerDisplayName,
      email: ownerEmail,
      external_id: ownerExternalID,
      department: ownerDepartment,
      title: ownerTitle,
      phone: ownerPhone,
      notes: ownerNotes
    }
    try {
      if (editingOwner?.uuid) await updateOwner(editingOwner.uuid, payload)
      else await createOwner(payload)
      ownerDialog?.close()
      await Promise.all([loadOwnerOptions(), loadOwners(), loadMachines()])
    } catch (err) {
      error = (err as Error).message || 'Failed to save owner'
    } finally {
      savingOwner = false
    }
  }

  async function confirmDeleteOwner() {
    if (!ownerToDelete?.uuid || !canDeleteInventory) return
    deletingOwner = true
    error = ''
    try {
      await deleteOwner(ownerToDelete.uuid)
      ownerToDelete = null
      await Promise.all([loadOwnerOptions(), loadOwners(), loadMachines()])
    } catch (err) {
      error = (err as Error).message || 'Failed to delete owner'
    } finally {
      deletingOwner = false
    }
  }

  async function changeMachinePage(page: number) {
    if (page < 1 || page > machinePageCount || page === currentMachinePage) return
    currentMachinePage = page
    await loadMachines()
  }

  async function changeOwnerPage(page: number) {
    if (page < 1 || page > ownerPageCount || page === currentOwnerPage) return
    currentOwnerPage = page
    await loadOwners()
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">Inventory</h1>
      <p class="text-light">Track device owners and internal asset IDs</p>
    </div>
    {#if canViewSettings}
      <button type="button" onclick={() => (bootstrapDialogOpen = true)}>Install osquery</button>
    {/if}
  </header>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if !ready}
    <Spinner />
  {:else}
    <ot-tabs bind:this={tabs} class="inventory-tabs" onot-tab-change={handleTabChange}>
      <div role="tablist" aria-label="Inventory sections">
        <button
          type="button"
          role="tab"
          aria-selected={activeTabIndex === 0}
          onclick={() => (activeTabIndex = 0)}
        >
          Devices
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activeTabIndex === 1}
          onclick={() => (activeTabIndex = 1)}
        >
          Owners
        </button>
      </div>

      <div role="tabpanel">
        <div class="vstack gap-3">
          <div class="row filter-row">
            <div class="col-5">
              <SearchInput bind:value={searchTerm} placeholder="Search inventory..." />
            </div>
            <div class="col-3">
              <SelectFilter
                options={ownerFilterOptions}
                bind:value={selectedOwner}
                label="Owner"
                allLabel="All owners"
              />
            </div>
            <div class="col-2">
              <SelectFilter
                options={[
                  { value: 'assigned', label: 'Assigned' },
                  { value: 'unassigned', label: 'Unassigned' }
                ]}
                bind:value={assignmentFilter}
                label="Assignment"
                allLabel="All devices"
              />
            </div>
            <div class="col-2">
              <SelectFilter
                options={allPlatforms}
                bind:value={selectedPlatform}
                label="Platform"
                allLabel="All platforms"
              />
            </div>
          </div>

          {#if loadingMachines}
            <Spinner />
          {:else}
            <div class="table">
              <table>
                <thead>
                  <tr>
                    <th>Status</th>
                    <th>Hostname</th>
                    <th>Owner</th>
                    <th>Tracking ID</th>
                    <th>Serial</th>
                    <th>Platform</th>
                    <th>Last Seen</th>
                    <th class="align-right"><span class="sr-only">Actions</span></th>
                  </tr>
                </thead>
                <tbody>
                  {#each machines as machine}
                    <tr>
                      <td>
                        <span class="badge" data-variant={statusVariant(machine)}>{statusLabel(machine)}</span>
                      </td>
                      <td><a href="/machines/{machine.uuid}">{machineHostname(machine)}</a></td>
                      <td>
                        {#if machine.inventory?.owner}
                          <strong>{machine.inventory.owner.display_name}</strong>
                          {#if machine.inventory.owner.email}
                            <p class="text-light">{machine.inventory.owner.email}</p>
                          {/if}
                        {:else}
                          <span class="text-light">Unassigned</span>
                        {/if}
                      </td>
                      <td>{machine.inventory?.internal_tracking_id || ''}</td>
                      <td>{machine.hardware_serial || ''}</td>
                      <td>{machine.platform || ''}</td>
                      <td>{formatTimestamp(machine.last_seen_at || machine.enrolled_at)}</td>
                    <td class="align-right">
                      {#if canUpdateInventory}
                        <ActionsMenu label={`Actions for ${machineHostname(machine)}`}>
                          <button role="menuitem" type="button" onclick={() => openInventoryDialog(machine)}>Edit inventory</button>
                        </ActionsMenu>
                      {/if}
                    </td>
                    </tr>
                  {:else}
                    <tr>
                      <td colspan="8" class="align-center text-light">No devices found</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>

            <footer class="hstack justify-between">
              <p class="text-light">
                Showing <strong>{machineStartResult}</strong> to <strong>{machineEndResult}</strong> of
                <strong>{machineTotalCount}</strong> devices
              </p>
              <Pagination
                currentPage={currentMachinePage}
                pageCount={machinePageCount}
                disabled={loadingMachines}
                label="Devices pagination"
                onPageChange={changeMachinePage}
              />
            </footer>
          {/if}
        </div>
      </div>

      <div role="tabpanel">
        <div class="vstack gap-3">
          <div class="hstack justify-between">
            <div>
              <h2 class="mb-2">Owners</h2>
              <p class="text-light">{ownerTotalCount} owners</p>
            </div>
            {#if canCreateInventory}
              <button type="button" onclick={openCreateOwner}>Create Owner</button>
            {/if}
          </div>

          <div class="row filter-row">
            <div class="col-5">
              <SearchInput bind:value={ownerSearchTerm} placeholder="Search owners..." />
            </div>
          </div>

          {#if loadingOwners}
            <Spinner />
          {:else}
            <div class="table">
              <table>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Email</th>
                    <th>Department</th>
                    <th class="align-right">Devices</th>
                    <th class="align-right"><span class="sr-only">Actions</span></th>
                  </tr>
                </thead>
                <tbody>
                  {#each owners as owner}
                    <tr>
                      <td><strong>{owner.display_name}</strong></td>
                      <td>{owner.email || ''}</td>
                      <td>{owner.department || ''}</td>
                      <td class="align-right">{owner.machine_count || 0}</td>
                    <td class="align-right">
                      {#if canUpdateInventory || canDeleteInventory}
                        <ActionsMenu label={`Actions for ${owner.display_name || 'owner'}`}>
                          {#if canUpdateInventory}
                            <button role="menuitem" type="button" onclick={() => openEditOwner(owner)}>Edit</button>
                          {/if}
                          {#if canUpdateInventory && canDeleteInventory}<hr />{/if}
                          {#if canDeleteInventory}
                            <button role="menuitem" type="button" onclick={() => (ownerToDelete = owner)}>Delete</button>
                          {/if}
                        </ActionsMenu>
                      {/if}
                    </td>
                    </tr>
                  {:else}
                    <tr>
                      <td colspan="5" class="align-center text-light">No owners found</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>

            <footer class="hstack justify-between">
              <p class="text-light">
                Showing <strong>{ownerStartResult}</strong> to <strong>{ownerEndResult}</strong> of
                <strong>{ownerTotalCount}</strong> owners
              </p>
              <Pagination
                currentPage={currentOwnerPage}
                pageCount={ownerPageCount}
                disabled={loadingOwners}
                label="Owners pagination"
                onPageChange={changeOwnerPage}
              />
            </footer>
          {/if}
        </div>
      </div>
    </ot-tabs>
  {/if}
</section>

<dialog bind:this={inventoryDialog} closedby="any">
  <form onsubmit={saveInventory}>
    <header>
      <h2>Edit Inventory</h2>
      {#if editingMachine}
        <p class="text-light">{machineHostname(editingMachine)}</p>
      {/if}
    </header>

    <div class="vstack">
      <label data-field>
        Owner
        <select bind:value={inventoryOwnerID}>
          <option value="">Unassigned</option>
          {#each ownerOptions as owner}
            <option value={owner.uuid}>{owner.display_name || owner.email || owner.uuid}</option>
          {/each}
        </select>
      </label>

      <label data-field>
        Internal tracking ID
        <input bind:value={internalTrackingID} placeholder="ASSET-10042" />
      </label>

      <label data-field>
        Notes
        <textarea bind:value={inventoryNotes} rows="3"></textarea>
      </label>
    </div>

    <footer class="hstack justify-end">
      <button type="button" class="outline" onclick={() => inventoryDialog?.close()}>Cancel</button>
      <button
        type="submit"
        disabled={savingInventory || loadingOwnerOptions}
        aria-busy={savingInventory ? 'true' : undefined}
      >
        {savingInventory ? 'Saving...' : 'Save'}
      </button>
    </footer>
  </form>
</dialog>

<dialog bind:this={ownerDialog} closedby="any">
  <form onsubmit={saveOwner}>
    <header>
      <h2>{editingOwner ? 'Edit Owner' : 'Create Owner'}</h2>
    </header>

    <div class="vstack">
      <label data-field>
        Display name
        <input bind:value={ownerDisplayName} required />
      </label>

      <label data-field>
        Email
        <input bind:value={ownerEmail} type="email" required />
      </label>

      <label data-field>
        External ID
        <input bind:value={ownerExternalID} />
      </label>

      <label data-field>
        Department
        <input bind:value={ownerDepartment} />
      </label>

      <label data-field>
        Title
        <input bind:value={ownerTitle} />
      </label>

      <label data-field>
        Phone
        <input bind:value={ownerPhone} />
      </label>

      <label data-field>
        Notes
        <textarea bind:value={ownerNotes} rows="3"></textarea>
      </label>
    </div>

    <footer class="hstack justify-end">
      <button type="button" class="outline" onclick={() => ownerDialog?.close()}>Cancel</button>
      <button type="submit" disabled={savingOwner} aria-busy={savingOwner ? 'true' : undefined}>
        {savingOwner ? 'Saving...' : 'Save'}
      </button>
    </footer>
  </form>
</dialog>

<ConfirmDialog
  open={!!ownerToDelete}
  title="Delete Owner"
  message="Delete this owner? Their devices will become unassigned."
  confirming={deletingOwner}
  confirmingLabel="Deleting..."
  onConfirm={confirmDeleteOwner}
  onCancel={() => (ownerToDelete = null)}
/>

<OsqueryBootstrapDialog bind:open={bootstrapDialogOpen} />

<style>
  .inventory-tabs {
    display: block;
  }

  .filter-row {
    align-items: end;
  }

  .filter-row :global(label[data-field]),
  .filter-row :global(.selectdropdown) {
    margin: 0;
  }

  .filter-row :global(input),
  .filter-row :global(.selectdropdown-trigger) {
    min-height: 2.5rem;
  }
</style>
