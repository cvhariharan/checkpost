<script lang="ts">
  import { page } from '$app/stores'
</script>

<section class="flex justify-center">
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
