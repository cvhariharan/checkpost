<script lang="ts">
  export let label = 'Actions'

  const menuId = `actions-menu-${Math.random().toString(36).slice(2, 10)}`
</script>

<ot-dropdown>
  <button
    type="button"
    class="icon ghost small"
    popovertarget={menuId}
    aria-label={label}
  >⋮</button>
  <menu popover id={menuId} role="menu">
    <slot {menuId} />
  </menu>
</ot-dropdown>
