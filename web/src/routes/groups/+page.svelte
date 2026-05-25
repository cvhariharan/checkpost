<script lang="ts">
  import { onMount } from 'svelte'
  import { deleteGroup as apiDeleteGroup, fetchGroups, type Group } from '$lib/api'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Pagination from '$lib/components/Pagination.svelte'
  import SearchInput from '$lib/components/SearchInput.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import GroupFormDialog from '$lib/components/GroupFormDialog.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'

  let loadedGroups: Group[] = []
  let currentPage = 1
  let pageCount = 1
  let totalCount = 0
  const countPerPage = 10
  let searchTerm = ''
  let error = ''
  let loading = true
  let formOpen = false
  let editingGroup: Group | null = null
  let deleteOpen = false
  let selectedGroup: Group | null = null
  let isDeleting = false

  $: groups = loadedGroups.filter((g) => {
    const search = searchTerm.trim().toLowerCase()
    return (
      !search ||
      (g.name || '').toLowerCase().includes(search) ||
      (g.description || '').toLowerCase().includes(search)
    )
  })
  $: startResult = loadedGroups.length === 0 ? 0 : (currentPage - 1) * countPerPage + 1
  $: endResult = Math.min(currentPage * countPerPage, totalCount)

  onMount(loadGroups)

  async function loadGroups() {
    loading = true
    error = ''
    try {
      const data = await fetchGroups({ page: currentPage, countPerPage })
      loadedGroups = data.groups || []
      pageCount = data.page_count || 1
      totalCount = data.total_count || loadedGroups.length
    } catch (err) {
      error = (err as Error).message || 'Failed to fetch groups'
    } finally {
      loading = false
    }
  }

  async function changePage(page: number) {
    if (page > 0 && page <= pageCount) {
      currentPage = page
      await loadGroups()
    }
  }

  function openCreate() {
    editingGroup = null
    formOpen = true
  }

  function openEdit(group: Group) {
    editingGroup = group
    formOpen = true
  }

  async function handleSaved() {
    formOpen = false
    await loadGroups()
  }

  function confirmDelete(group: Group) {
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
  <header class="hstack justify-between">
    <div>
      <h1>Groups</h1>
      <p class="text-light">Organize machines and target policies to specific collections</p>
    </div>
    <button type="button" onclick={openCreate}>Create Group</button>
  </header>

  <div class="row">
    <div class="col-6">
      <SearchInput bind:value={searchTerm} placeholder="Search groups..." />
    </div>
  </div>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if loading}
    <Spinner />
  {:else}
    <div class="table">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Description</th>
            <th class="align-right">Machines</th>
            <th class="align-right">Policies</th>
            <th class="align-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each groups as group}
            <tr>
              <td><strong>{group.name || 'Untitled'}</strong></td>
              <td>{group.description || ''}</td>
              <td class="align-right">{group.machine_count || 0}</td>
              <td class="align-right">{group.policy_count || 0}</td>
              <td class="align-right">
                <menu class="buttons">
                  <li><button type="button" class="small outline" onclick={() => openEdit(group)}>Edit</button></li>
                  <li>
                    <button
                      type="button"
                      class="small outline"
                      data-variant="danger"
                      onclick={() => confirmDelete(group)}
                    >
                      Delete
                    </button>
                  </li>
                </menu>
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
      <Pagination {currentPage} {pageCount} onPageChange={changePage} />
    </footer>
  {/if}
</section>

<GroupFormDialog
  open={formOpen}
  group={editingGroup}
  onClose={() => (formOpen = false)}
  onSaved={handleSaved}
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
