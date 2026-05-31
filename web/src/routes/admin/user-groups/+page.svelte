<script lang="ts">
  import { onMount } from 'svelte'
  import {
    fetchUserGroups,
    createUserGroup,
    updateUserGroup,
    deleteUserGroup as apiDeleteUserGroup,
    fetchUserGroupMembers,
    addUserGroupMember,
    removeUserGroupMember,
    fetchUsers,
    fetchRoles,
    fetchRoleBindings,
    createRoleBinding,
    deleteRoleBinding,
    type UserGroup,
    type UserGroupMember,
    type RoleBinding,
    type User
  } from '$lib/api'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import ActionsMenu from '$lib/components/ActionsMenu.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'
  import { canFrom, me } from '$lib/auth'

  let groups = $state<UserGroup[]>([])
  let groupRoles = $state<Record<string, string>>({})
  let loading = $state(true)
  let error = $state('')

  let roleOptions = $state(['admin', 'operator', 'analyst', 'viewer'])
  let roleBindings = $state<RoleBinding[]>([])
  let selectedRole = $state('viewer')
  let loadingRole = $state(false)

  let formDialog = $state<HTMLDialogElement>()
  let editing = $state<UserGroup | null>(null)
  let name = $state('')
  let description = $state('')
  let oidcClaim = $state('')
  let saving = $state(false)
  let formError = $state('')

  let deleteOpen = $state(false)
  let selected = $state<UserGroup | null>(null)
  let isDeleting = $state(false)

  let membersDialog = $state<HTMLDialogElement>()
  let membersGroup = $state<UserGroup | null>(null)
  let members = $state<UserGroupMember[]>([])
  let allUsers = $state<User[]>([])
  let addUserId = $state('')
  let membersError = $state('')
  let membersSaving = $state(false)
  const canCreateUserGroup = $derived(canFrom($me, 'user_group', 'create'))
  const canUpdateUserGroup = $derived(canFrom($me, 'user_group', 'update'))
  const canDeleteUserGroup = $derived(canFrom($me, 'user_group', 'delete'))
  const canViewRoleBindings = $derived(canFrom($me, 'role_binding', 'view'))
  const canManageRoleBindings = $derived(
    canViewRoleBindings &&
      canFrom($me, 'role_binding', 'create') &&
      canFrom($me, 'role_binding', 'delete')
  )

  onMount(() => {
    void loadRoles()
    void load()
  })

  async function loadRoles() {
    try {
      const data = await fetchRoles()
      const names = data.roles.map((role) => role.name)
      if (names.length) roleOptions = names
    } catch {
      // Keep the built-in fallback list if the catalog cannot be loaded.
    }
  }

  async function load() {
    loading = true
    error = ''
    try {
      const data = await fetchUserGroups({ page: 1, countPerPage: 200 })
      groups = data.user_groups || []
      await loadRoleSummaries(groups)
    } catch (err) {
      error = (err as Error).message || 'Failed to load user groups'
    } finally {
      loading = false
    }
  }

  async function loadRoleSummaries(items: UserGroup[]) {
    if ($me && !canViewRoleBindings) {
      groupRoles = {}
      return
    }
    try {
      const entries = await Promise.all(
        items.map(async (group) => {
          const data = await fetchRoleBindings('usergroup', group.uuid)
          return [group.uuid, roleSummary(data.bindings || [])] as const
        })
      )
      groupRoles = Object.fromEntries(entries)
    } catch {
      groupRoles = {}
    }
  }

  function roleSummary(bindings: RoleBinding[]) {
    if (!bindings.length) return 'No role'
    return bindings.map((binding) => binding.role).join(', ')
  }

  function selectedRoleFrom(bindings: RoleBinding[]) {
    return bindings[0]?.role || ''
  }

  function openCreate() {
    if (!canCreateUserGroup) return
    editing = null
    name = ''
    description = ''
    oidcClaim = ''
    roleBindings = []
    selectedRole = 'viewer'
    loadingRole = false
    formError = ''
    formDialog?.showModal()
  }

  async function openEdit(group: UserGroup) {
    if (!canUpdateUserGroup) return
    editing = group
    name = group.name
    description = group.description || ''
    oidcClaim = group.oidc_claim_value || ''
    roleBindings = []
    selectedRole = ''
    loadingRole = true
    formError = ''
    formDialog?.showModal()
    if (canViewRoleBindings) {
      try {
        const data = await fetchRoleBindings('usergroup', group.uuid)
        roleBindings = data.bindings || []
        selectedRole = selectedRoleFrom(roleBindings)
      } catch (err) {
        formError = (err as Error).message || 'Failed to load role'
      } finally {
        loadingRole = false
      }
    } else {
      loadingRole = false
    }
  }

  async function save(event: SubmitEvent) {
    event.preventDefault()
    saving = true
    formError = ''
    try {
      if (editing ? !canUpdateUserGroup : !canCreateUserGroup) return
      const payload = { name, description, oidc_claim_value: oidcClaim }
      const saved = editing
        ? await updateUserGroup(editing.uuid, payload)
        : await createUserGroup(payload)
      await syncRole(saved.uuid)
      formDialog?.close()
      await load()
    } catch (err) {
      formError = (err as Error).message || 'Failed to save user group'
    } finally {
      saving = false
    }
  }

  async function syncRole(groupId: string) {
    if (!canManageRoleBindings) return
    await Promise.all(roleBindings.map((binding) => deleteRoleBinding(binding.uuid)))
    if (selectedRole) {
      await createRoleBinding({
        subject_type: 'usergroup',
        subject_id: groupId,
        role: selectedRole
      })
    }
  }

  function confirmDelete(group: UserGroup) {
    if (!canDeleteUserGroup) return
    selected = group
    deleteOpen = true
  }

  async function doDelete() {
    if (!selected) return
    isDeleting = true
    try {
      await apiDeleteUserGroup(selected.uuid)
      deleteOpen = false
      selected = null
      await load()
    } catch (err) {
      error = (err as Error).message || 'Failed to delete user group'
    } finally {
      isDeleting = false
    }
  }

  async function openMembers(group: UserGroup) {
    membersGroup = group
    members = []
    addUserId = ''
    membersError = ''
    membersDialog?.showModal()
    try {
      const [m, u] = await Promise.all([
        fetchUserGroupMembers(group.uuid),
        fetchUsers({ page: 1, countPerPage: 500 })
      ])
      members = m.members || []
      allUsers = u.users || []
    } catch (err) {
      membersError = (err as Error).message || 'Failed to load members'
    }
  }

  const memberIds = $derived(members.map((m) => m.user_uuid))
  const candidateUsers = $derived(allUsers.filter((u) => !memberIds.includes(u.uuid)))

  async function addMember() {
    if (!membersGroup || !addUserId || !canUpdateUserGroup) return
    membersSaving = true
    membersError = ''
    try {
      await addUserGroupMember(membersGroup.uuid, addUserId)
      addUserId = ''
      const m = await fetchUserGroupMembers(membersGroup.uuid)
      members = m.members || []
      await load()
    } catch (err) {
      membersError = (err as Error).message || 'Failed to add member'
    } finally {
      membersSaving = false
    }
  }

  async function removeMember(userUUID: string) {
    if (!membersGroup || !canUpdateUserGroup) return
    membersSaving = true
    membersError = ''
    try {
      await removeUserGroupMember(membersGroup.uuid, userUUID)
      const m = await fetchUserGroupMembers(membersGroup.uuid)
      members = m.members || []
      await load()
    } catch (err) {
      membersError = (err as Error).message || 'Failed to remove member'
    } finally {
      membersSaving = false
    }
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">User Groups</h1>
      <p class="text-light">Collections of users for role assignment. Membership can sync from the OIDC groups claim.</p>
    </div>
    {#if canCreateUserGroup}
      <button type="button" onclick={openCreate}>Create User Group</button>
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
            <th>Description</th>
            <th>OIDC claim</th>
            {#if canViewRoleBindings}<th>Role</th>{/if}
            <th class="align-right">Members</th>
            <th class="align-right"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {#each groups as group}
            <tr>
              <td>
                {#if canUpdateUserGroup}
                  <button type="button" class="cell-link" onclick={() => openEdit(group)}>{group.name}</button>
                {:else}
                  <strong>{group.name}</strong>
                {/if}
              </td>
              <td>{group.description || ''}</td>
              <td>{group.oidc_claim_value || ''}</td>
              {#if canViewRoleBindings}<td>{groupRoles[group.uuid] || 'No role'}</td>{/if}
              <td class="align-right">{group.member_count || 0}</td>
              <td class="align-right">
                <ActionsMenu label={`Actions for ${group.name}`}>
                  {#if canUpdateUserGroup}
                    <button role="menuitem" type="button" onclick={() => openEdit(group)}>Edit</button>
                  {/if}
                  <button role="menuitem" type="button" onclick={() => openMembers(group)}>Members</button>
                  {#if canDeleteUserGroup}<hr />{/if}
                  {#if canDeleteUserGroup}
                    <button role="menuitem" type="button" onclick={() => confirmDelete(group)}>Delete</button>
                  {/if}
                </ActionsMenu>
              </td>
            </tr>
          {:else}
            <tr><td colspan={canViewRoleBindings ? 6 : 5} class="align-center text-light">No user groups found</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</section>

<dialog bind:this={formDialog} closedby="any">
  <form onsubmit={save}>
    <header><h2>{editing ? 'Edit User Group' : 'Create User Group'}</h2></header>
    <ErrorMessage message={formError} onClose={() => (formError = '')} />
    <div class="vstack">
      <label data-field>
        Name
        <input bind:value={name} required placeholder="Group name" />
      </label>
      <label data-field>
        Description
        <textarea bind:value={description} rows="2" placeholder="What this group represents"></textarea>
      </label>
      <label data-field>
        OIDC claim value
        <input bind:value={oidcClaim} placeholder="Matches an entry in the groups claim (optional)" />
      </label>
      {#if canManageRoleBindings}
        <label data-field>
          Role
          <select bind:value={selectedRole} disabled={loadingRole}>
            <option value="">No role</option>
            {#each roleOptions as role}
              <option value={role}>{role}</option>
            {/each}
          </select>
        </label>
      {/if}
    </div>
    <footer>
      <button type="button" class="outline" onclick={() => formDialog?.close()}>Cancel</button>
      <button type="submit" disabled={saving || loadingRole} aria-busy={saving ? 'true' : undefined}>
        {saving ? 'Saving...' : editing ? 'Update' : 'Create'}
      </button>
    </footer>
  </form>
</dialog>

<dialog bind:this={membersDialog} closedby="any">
  <form method="dialog">
    <header><h2>Members — {membersGroup?.name}</h2></header>
    <ErrorMessage message={membersError} onClose={() => (membersError = '')} />
    <div class="vstack gap-3">
      {#if canUpdateUserGroup}
        <div class="hstack gap-2 add-row">
          <select bind:value={addUserId} disabled={membersSaving}>
            <option value="">Select a user to add</option>
            {#each candidateUsers as user}
              <option value={user.uuid}>{user.username}</option>
            {/each}
          </select>
          <button type="button" onclick={addMember} disabled={!addUserId || membersSaving}>Add</button>
        </div>
      {/if}
      <div class="table">
        <table>
          <thead>
            <tr><th>Username</th><th>Source</th><th class="align-right"><span class="sr-only">Actions</span></th></tr>
          </thead>
          <tbody>
            {#each members as member}
              <tr>
                <td>{member.username}</td>
                <td>{member.source}</td>
                <td class="align-right">
                  {#if member.source === 'manual' && canUpdateUserGroup}
                    <button
                      type="button"
                      class="small outline"
                      data-variant="danger"
                      disabled={membersSaving}
                      onclick={() => removeMember(member.user_uuid)}
                    >
                      Remove
                    </button>
                  {:else}
                    <span class="text-light">{member.source === 'manual' ? 'manual' : 'synced'}</span>
                  {/if}
                </td>
              </tr>
            {:else}
              <tr><td colspan="3" class="align-center text-light">No members</td></tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
    <footer>
      <button type="button" class="outline" onclick={() => membersDialog?.close()}>Close</button>
    </footer>
  </form>
</dialog>

<ConfirmDialog
  bind:open={deleteOpen}
  title="Delete User Group"
  message="Delete this user group? Memberships and role bindings are removed."
  confirming={isDeleting}
  confirmingLabel="Deleting..."
  onConfirm={doDelete}
  onCancel={() => (selected = null)}
/>

<style>
  .add-row select {
    flex: 1;
  }
</style>
