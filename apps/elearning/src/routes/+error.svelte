<script lang="ts">
  import { page } from '$app/state';
  import { IdentityError } from '@algohub/ui';
  import '../app.css';
</script>

<IdentityError
  status={page.status}
  message={page.error?.message ?? 'Permintaan tidak dapat diproses.'}
  path={page.url.pathname}
/>
