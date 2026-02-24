<script lang="ts">
  import * as UISidebar from '$lib/components/ui/sidebar/index.js';
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { createCreateOrganization } from '$lib/api/endpoints/organizations/organizations';

  import { goto } from '$app/navigation';
  import { getSession } from '$lib/api/endpoints/auth/auth';
  import { sessionStore } from '$lib/stores/session';
  import { onMount } from 'svelte';
  import { Sidebar } from './_components/sidebar';

  let {children} = $props();
  let loading = $state(true);
  let onboardingRequired = $state(false);
  let onboardingError = $state('');
  let onboardingOrgName = $state('My Organization');

  const createOrgMutation = createCreateOrganization();

  async function loadSession() {
    try {
      const res = await getSession();
      if (res.status !== 200) {
        goto('/sign-in');
        return;
      }

      sessionStore.set(res.data);
      onboardingRequired = !res.data.active_organization?.id;
      if (onboardingRequired && res.data.user?.name) {
        onboardingOrgName = `${res.data.user.name}'s Organization`;
      }
    } catch {
      goto('/sign-in');
    } finally {
      loading = false;
    }
  }

  function createOrganization() {
    const name = onboardingOrgName.trim();
    if (!name) {
      onboardingError = 'Organization name is required';
      return;
    }

    onboardingError = '';
    createOrgMutation.mutate(
      {
        data: {
          name,
          set_as_active_organization: true
        }
      },
      {
        onSuccess: async (res) => {
          if (res.status !== 200) {
            onboardingError = (res.data as any)?.detail ?? 'Failed to create organization';
            return;
          }
          await loadSession();
        },
        onError: () => {
          onboardingError = 'Failed to create organization';
        }
      }
    );
  }

  onMount(async () => {
    await loadSession();
  });
</script>

{#if loading}
  <div class="flex h-screen items-center justify-center">
    <div class="text-muted-foreground text-sm">Loading...</div>
  </div>
{:else if onboardingRequired}
  <div class="flex h-screen items-center justify-center p-4">
    <div class="bg-card w-full max-w-md rounded-xl border p-6 shadow-sm">
      <h1 class="text-xl font-semibold">Set up your organization</h1>
      <p class="text-muted-foreground mt-2 text-sm">
        You need an organization before accessing logs, traces, and metrics.
      </p>

      <div class="mt-5 space-y-3">
        <label class="text-sm font-medium" for="onboarding-org-name">Organization name</label>
        <Input id="onboarding-org-name" bind:value={onboardingOrgName} placeholder="My Organization" />
      </div>

      {#if onboardingError}
        <div class="text-destructive bg-destructive/10 mt-4 rounded-md px-3 py-2 text-sm">
          {onboardingError}
        </div>
      {/if}

      <div class="mt-5 flex justify-end">
        <Button onclick={createOrganization} disabled={createOrgMutation.isPending}>
          {#if createOrgMutation.isPending}
            Creating...
          {:else}
            Continue
          {/if}
        </Button>
      </div>
    </div>
  </div>
{:else}
  <UISidebar.Provider>
    <Sidebar />
    <UISidebar.Inset>
      {@render children()}
    </UISidebar.Inset>
  </UISidebar.Provider>
{/if}
