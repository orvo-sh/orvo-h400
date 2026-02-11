<script lang="ts">
  import * as UISidebar from "$lib/components/ui/sidebar/index.js";

  import { goto } from '$app/navigation';
  import { getSession } from '$lib/api/endpoints/auth/auth';
  import { sessionStore } from '$lib/stores/session';
  import { onMount } from 'svelte';
  import { Sidebar } from "./_components/sidebar";

  let {children} = $props();
  let loading = $state(true);

  onMount(async () => {
    try {
      const res = await getSession();
      if (res.status === 200) {
        sessionStore.set(res.data);
        loading = false;
      } else {
        goto('/sign-in');
      }
    } catch {
      goto('/sign-in');
    }
  });
</script>

{#if loading}
  <div class="flex h-screen items-center justify-center">
    <div class="text-muted-foreground text-sm">Loading...</div>
  </div>
{:else}
  <UISidebar.Provider>
    <Sidebar />
    <UISidebar.Inset>
      {@render children()}
    </UISidebar.Inset>
  </UISidebar.Provider>
{/if}
