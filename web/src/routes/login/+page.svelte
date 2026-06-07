<script lang="ts">
  import { onMount } from 'svelte'
  import { page } from '$app/state'
  import { fetchProviders, login, type Providers } from '$lib/api'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Logo from '$lib/components/Logo.svelte'

  let providers = $state<Providers>({ password: true, sso: { enabled: false, label: '' } })
  let username = $state('')
  let password = $state('')
  let error = $state('')
  let submitting = $state(false)

  const redirectURL = $derived(sanitizeRedirect(page.url.searchParams.get('redirect_url')))

  function sanitizeRedirect(value: string | null): string {
    if (!value) return '/'
    if (value[0] !== '/' || value[1] === '/' || value[1] === '\\') return '/'
    return value
  }

  onMount(async () => {
    try {
      providers = await fetchProviders()
    } catch {
      // default to password-only on failure
    }
  })

  async function handleSubmit(event: SubmitEvent) {
    event.preventDefault()
    submitting = true
    error = ''
    try {
      await login(username, password)
      window.location.assign(redirectURL)
    } catch (err) {
      error = (err as Error).message || 'Login failed'
    } finally {
      submitting = false
    }
  }

  function ssoLogin() {
    window.location.assign(`/login/oidc?redirect_url=${encodeURIComponent(redirectURL)}`)
  }
</script>

<main class="login-main">
  <div class="login-card card">
    <header class="login-header mb-4">
      <h1 class="login-title mb-0"><Logo size="lg" /></h1>
      <p class="text-light mb-0">Sign in to continue</p>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <form onsubmit={handleSubmit} class="vstack gap-3">
      <label data-field>
        Username
        <input bind:value={username} required autocomplete="username" placeholder="Username" />
      </label>
      <label data-field>
        Password
        <input
          type="password"
          bind:value={password}
          required
          autocomplete="current-password"
          placeholder="Password"
        />
      </label>
      <button type="submit" disabled={submitting} aria-busy={submitting ? 'true' : undefined}>
        {submitting ? 'Signing in...' : 'Sign in'}
      </button>
    </form>

    {#if providers.sso?.enabled}
      <div class="sso-divider"><span>or</span></div>
      <button type="button" class="outline w-full" onclick={ssoLogin}>
        {providers.sso.label || 'Sign in with SSO'}
      </button>
    {/if}
  </div>
</main>

<style>
  .login-main {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    padding: var(--space-4);
  }
  .login-card {
    width: min(24rem, 100%);
  }
  .login-header {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-1);
    text-align: center;
  }
  .login-title {
    display: flex;
    justify-content: center;
    width: 100%;
    line-height: 1;
  }
  .w-full {
    width: 100%;
  }
  .sso-divider {
    display: flex;
    align-items: center;
    text-align: center;
    color: var(--fg-muted, var(--text-light));
    margin: var(--space-3) 0;
  }
  .sso-divider::before,
  .sso-divider::after {
    content: '';
    flex: 1;
    border-bottom: 1px solid var(--border);
  }
  .sso-divider span {
    padding: 0 var(--space-2);
  }
</style>
