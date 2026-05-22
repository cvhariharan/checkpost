<script>
  import { link } from '@/routes.js'

  export let params = {}
  export let query = {}

  $: code = query.code || params.code || '404'
  $: message = query.message || params.message || 'Page not found or an error occurred.'
</script>

<section class="flex justify-center">
  <article class="card">
    <header>
      <h1>{code}</h1>
      <p>{message}</p>
    </header>
    <footer>
      <a href="/machines" use:link={'/machines'} class="button">Return Home</a>
    </footer>
  </article>
</section>
