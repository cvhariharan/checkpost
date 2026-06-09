<script lang="ts">
  import { untrack } from 'svelte'
  import { page } from '$app/state'
  import { replaceState } from '$app/navigation'
  import { deleteGroup as apiDeleteGroup, fetchGroups, type Group } from '$lib/api'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import SearchInput from '$lib/components/SearchInput.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import GroupFormDialog from '$lib/components/GroupFormDialog.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'
  import ActionsMenu from '$lib/components/ActionsMenu.svelte'
  import { canFrom, me } from '$lib/auth'

  let loadedGroups = $state<Group[]>([])
  const currentPage = $derived(Math.max(1, Number(page.url.searchParams.get('page')) || 1))
  let pageCount = $state(1)
  let totalCount = $state(0)
  const countPerPage = 10
  let searchTerm = $state('')
  let error = $state('')
  let loading = $state(true)
  let formOpen = $state(false)
  let editingGroup = $state<Group | null>(null)
  let deleteOpen = $state(false)
  let selectedGroup = $state<Group | null>(null)
  let isDeleting = $state(false)

  let initialized = $state(false)
  let previousSearch = ''
  let searchTimer: ReturnType<typeof setTimeout> | undefined

  $effect(() => {
    if (!initialized || searchTerm === previousSearch) return
    previousSearch = searchTerm
    if (currentPage !== 1) {
      const url = new URL(page.url)
      url.searchParams.delete('page')
      replaceState(url, {})
    }
    clearTimeout(searchTimer)
    searchTimer = setTimeout(() => void loadGroups(), 250)
  })

  const startResult = $derived(loadedGroups.length === 0 ? 0 : (currentPage - 1) * countPerPage + 1)
  const endResult = $derived(Math.min(currentPage * countPerPage, totalCount))
  const canCreateGroup = $derived(canFrom($me, 'machine_group', 'create'))
  const canUpdateGroup = $derived(canFrom($me, 'machine_group', 'update'))
  const canDeleteGroup = $derived(canFrom($me, 'machine_group', 'delete'))

  $effect(() => {
    currentPage
    untrack(() => void loadGroups())
  })

  async function loadGroups() {
    loading = true
    error = ''
    try {
      const data = await fetchGroups({ page: currentPage, countPerPage, query: searchTerm })
      loadedGroups = data.groups || []
      pageCount = data.page_count || 1
      totalCount = data.total_count || loadedGroups.length
    } catch (err) {
      error = (err as Error).message || 'Failed to fetch groups'
    } finally {
      previousSearch = searchTerm
      initialized = true
      loading = false
    }
  }

  function openCreate() {
    if (!canCreateGroup) return
    editingGroup = null
    formOpen = true
  }

  function openEdit(group: Group) {
    if (!canUpdateGroup) return
    editingGroup = group
    formOpen = true
  }

  async function handleSaved() {
    formOpen = false
    await loadGroups()
  }

  function confirmDelete(group: Group) {
    if (!canDeleteGroup) return
    selectedGroup = group
    deleteOpen = true
  }

  async function deleteGroup() {
    if (!selectedGroup) return
    isDeleting = true
    error = ''
    try {
      await apiDeleteGroup(selectedGroup.uuid)
      deleteOpen = false
      selectedGroup = null
      await loadGroups()
    } catch (err) {
      error = (err as Error).message || 'Failed to delete group'
    } finally {
      isDeleting = false
    }
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">Groups</h1>
      <p class="text-light">Organize machines and target policies to specific collections</p>
    </div>
    {#if canCreateGroup}
      <button type="button" onclick={openCreate}>Create Group</button>
    {/if}
  </header>

  <div class="row">
    <div class="col-6">
      <SearchInput bind:value={searchTerm} placeholder="Search groups..." />
    </div>
  </div>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if loading}
    <Spinner fill />
  {:else}
    <div class="table">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Description</th>
            <th class="align-right">Machines</th>
            <th class="align-right">Policies</th>
            <th class="align-right"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {#each loadedGroups as group}
            <tr>
              <td>
                {#if canUpdateGroup}
                  <button type="button" class="cell-link" onclick={() => openEdit(group)}>
                    {group.name || 'Untitled'}
                  </button>
                {:else}
                  <strong>{group.name || 'Untitled'}</strong>
                {/if}
              </td>
              <td>{group.description || ''}</td>
              <td class="align-right">{group.machine_count || 0}</td>
              <td class="align-right">{group.policy_count || 0}</td>
              <td class="align-right">
                {#if canUpdateGroup || canDeleteGroup}
                  <ActionsMenu label={`Actions for ${group.name || 'group'}`}>
                    {#if canUpdateGroup}
                      <button role="menuitem" type="button" onclick={() => openEdit(group)}>Edit</button>
                    {/if}
                    {#if canUpdateGroup && canDeleteGroup}<hr />{/if}
                    {#if canDeleteGroup}
                      <button role="menuitem" type="button" onclick={() => confirmDelete(group)}>Delete</button>
                    {/if}
                  </ActionsMenu>
                {/if}
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="5" class="align-center text-light">No groups found</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <footer class="hstack justify-between">
      <p class="text-light">
        Showing <strong>{startResult}</strong> to <strong>{endResult}</strong> of
        <strong>{totalCount}</strong> results
      </p>
      <Pagination {currentPage} {pageCount} param="page" />
    </footer>
  {/if}
</section>

<GroupFormDialog
  open={formOpen}
  group={editingGroup}
  canManageMembers={canUpdateGroup}
  onClose={() => (formOpen = false)}
  onSaved={handleSaved}
  onChanged={loadGroups}
/>

<ConfirmDialog
  bind:open={deleteOpen}
  title="Delete Group"
  message="Are you sure you want to delete this group? Machines and policies will be detached automatically."
  confirming={isDeleting}
  confirmingLabel="Deleting..."
  onConfirm={deleteGroup}
  onCancel={() => (selectedGroup = null)}
/>
