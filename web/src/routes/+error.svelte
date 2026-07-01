<script lang="ts">
  import { page } from '$app/stores'
</script>

<section class="error-page flex justify-center">
  <article class="card">
    <header>
      <h1>{$page.status}</h1>
      <p>{$page.error?.message || 'Page not found or an error occurred.'}</p>
    </header>
    <footer>
      <a href="/inventory" class="button">Return Home</a>
    </footer>
  </article>
</section>

<style>
  .error-page {
    align-items: center;
    min-height: 60vh;
  }
</style>
