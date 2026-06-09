<script lang="ts">
  import { onMount } from 'svelte'
  import {
    fetchUsers,
    createUser,
    updateUser,
    deleteUser as apiDeleteUser,
    fetchRoles,
    fetchRoleBindings,
    createRoleBinding,
    deleteRoleBinding,
    type RoleBinding,
    type User
  } from '$lib/api'
  import { formatTimestamp } from '$lib/util'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import ActionsMenu from '$lib/components/ActionsMenu.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'
  import { canFrom, me } from '$lib/auth'

  let users = $state<User[]>([])
  let userRoles = $state<Record<string, string>>({})
  let loading = $state(true)
  let error = $state('')

  let roleOptions = $state(['admin', 'operator', 'analyst', 'viewer'])
  let roleBindings = $state<RoleBinding[]>([])
  let selectedRole = $state('viewer')
  let loadingRole = $state(false)

  let dialog = $state<HTMLDialogElement>()
  let editing = $state<User | null>(null)
  let name = $state('')
  let email = $state('')
  let disabled = $state(false)
  let saving = $state(false)
  let formError = $state('')

  let deleteOpen = $state(false)
  let selected = $state<User | null>(null)
  let isDeleting = $state(false)
  const canCreateUser = $derived(canFrom($me, 'user', 'create'))
  const canUpdateUser = $derived(canFrom($me, 'user', 'update'))
  const canDeleteUser = $derived(canFrom($me, 'user', 'delete'))
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
      const data = await fetchUsers({ page: 1, countPerPage: 200 })
      users = data.users || []
      await loadRoleSummaries(users)
    } catch (err) {
      error = (err as Error).message || 'Failed to load users'
    } finally {
      loading = false
    }
  }

  async function loadRoleSummaries(items: User[]) {
    if ($me && !canViewRoleBindings) {
      userRoles = {}
      return
    }
    try {
      const entries = await Promise.all(
        items.map(async (user) => {
          const data = await fetchRoleBindings('user', user.uuid)
          return [user.uuid, roleSummary(data.bindings || [])] as const
        })
      )
      userRoles = Object.fromEntries(entries)
    } catch {
      userRoles = {}
    }
  }

  function roleSummary(bindings: RoleBinding[]) {
    if (!bindings.length) return 'No role'
    return bindings.map((binding) => binding.role).join(', ')
  }

  function selectedRoleFrom(bindings: RoleBinding[]) {
    return bindings[0]?.role || ''
  }

  function isSystemUser(user: User) {
    return user.login_type === 'standard'
  }

  function openCreate() {
    if (!canCreateUser) return
    editing = null
    name = ''
    email = ''
    disabled = false
    roleBindings = []
    selectedRole = 'viewer'
    loadingRole = false
    formError = ''
    dialog?.showModal()
  }

  async function openEdit(user: User) {
    if (isSystemUser(user) || !canUpdateUser) return
    editing = user
    name = user.name || ''
    email = user.email || ''
    disabled = !!user.disabled
    roleBindings = []
    selectedRole = ''
    loadingRole = true
    formError = ''
    dialog?.showModal()
    if (canViewRoleBindings) {
      try {
        const data = await fetchRoleBindings('user', user.uuid)
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
      let saved: User
      if (editing) {
        if (!canUpdateUser) return
        saved = await updateUser(editing.uuid, { name, email, disabled })
      } else {
        if (!canCreateUser) return
        saved = await createUser({ name, email })
      }
      await syncRole(saved.uuid)
      dialog?.close()
      await load()
    } catch (err) {
      formError = (err as Error).message || 'Failed to save user'
    } finally {
      saving = false
    }
  }

  async function syncRole(userId: string) {
    if (!canManageRoleBindings) return
    await Promise.all(roleBindings.map((binding) => deleteRoleBinding(binding.uuid)))
    if (selectedRole) {
      await createRoleBinding({
        subject_type: 'user',
        subject_id: userId,
        role: selectedRole
      })
    }
  }

  function confirmDelete(user: User) {
    if (isSystemUser(user) || !canDeleteUser) return
    selected = user
    deleteOpen = true
  }

  async function doDelete() {
    if (!selected) return
    isDeleting = true
    try {
      await apiDeleteUser(selected.uuid)
      deleteOpen = false
      selected = null
      await load()
    } catch (err) {
      error = (err as Error).message || 'Failed to delete user'
    } finally {
      isDeleting = false
    }
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">Users</h1>
      <p class="text-light">SSO accounts and the bootstrap admin</p>
    </div>
    {#if canCreateUser}
      <button type="button" onclick={openCreate}>Create User</button>
    {/if}
  </header>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if loading}
    <Spinner fill />
  {:else}
    <div class="table">
      <table>
        <thead>
          <tr>
            <th>Username</th>
            <th>Name</th>
            <th>Type</th>
            {#if canViewRoleBindings}<th>Role</th>{/if}
            <th>Status</th>
            <th>Last login</th>
            <th class="align-right"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {#each users as user}
            <tr>
              <td>
                {#if isSystemUser(user) || !canUpdateUser}
                  <strong>{user.username}</strong>
                {:else}
                  <button type="button" class="cell-link" onclick={() => openEdit(user)}>
                    {user.username}
                  </button>
                {/if}
              </td>
              <td>{user.name || ''}</td>
              <td>{user.login_type || ''}</td>
              {#if canViewRoleBindings}<td>{userRoles[user.uuid] || 'No role'}</td>{/if}
              <td>{user.disabled ? 'Disabled' : 'Active'}</td>
              <td>{user.last_login_at ? formatTimestamp(user.last_login_at) : 'Never'}</td>
              <td class="align-right">
                {#if isSystemUser(user)}
                  <span class="text-light">System</span>
                {:else if !canUpdateUser && !canDeleteUser}
                  <span class="text-light">—</span>
                {:else}
                  <ActionsMenu label={`Actions for ${user.username}`}>
                    {#if canUpdateUser}
                      <button role="menuitem" type="button" onclick={() => openEdit(user)}>Edit</button>
                    {/if}
                    {#if canUpdateUser && canDeleteUser}<hr />{/if}
                    {#if canDeleteUser}
                      <button role="menuitem" type="button" onclick={() => confirmDelete(user)}>Delete</button>
                    {/if}
                  </ActionsMenu>
                {/if}
              </td>
            </tr>
          {:else}
            <tr><td colspan={canViewRoleBindings ? 7 : 6} class="align-center text-light">No users found</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</section>

<dialog bind:this={dialog} closedby="any">
  <form onsubmit={save}>
    <header><h2>{editing ? 'Edit User' : 'Create User'}</h2></header>
    <ErrorMessage message={formError} onClose={() => (formError = '')} />
    <div class="vstack">
      <label data-field>
        Name
        <input bind:value={name} placeholder="Full name" />
      </label>
      <label data-field>
        Email
        <input
          type="email"
          bind:value={email}
          required={editing?.login_type !== 'standard'}
          placeholder="user@example.com"
        />
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
      {#if editing}
        <label data-field class="hstack gap-2 align-center">
          <input type="checkbox" bind:checked={disabled} />
          Disabled
        </label>
      {/if}
    </div>
    <footer>
      <button type="button" class="outline" onclick={() => dialog?.close()}>Cancel</button>
      <button type="submit" class="gap-1" disabled={saving || loadingRole} aria-busy={saving ? 'true' : undefined} data-spinner="small">
        {saving ? 'Saving...' : editing ? 'Update' : 'Create'}
      </button>
    </footer>
  </form>
</dialog>

<ConfirmDialog
  bind:open={deleteOpen}
  title="Delete User"
  message="Delete this user? Their memberships and role bindings are removed."
  confirming={isDeleting}
  confirmingLabel="Deleting..."
  onConfirm={doDelete}
  onCancel={() => (selected = null)}
/>
