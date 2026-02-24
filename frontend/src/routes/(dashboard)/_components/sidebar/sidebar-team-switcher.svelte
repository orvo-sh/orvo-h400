<script lang="ts">
	import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index.js";
	import * as Sidebar from "$lib/components/ui/sidebar/index.js";
	import { useSidebar } from "$lib/components/ui/sidebar/index.js";
	import * as Dialog from "$lib/components/ui/dialog/index.js";
	import { Input } from "$lib/components/ui/input/index.js";
	import { Button } from "$lib/components/ui/button/index.js";
	import ChevronsUpDownIcon from "@lucide/svelte/icons/chevrons-up-down";
	import PlusIcon from "@lucide/svelte/icons/plus";
	import GalleryVerticalEndIcon from '@lucide/svelte/icons/gallery-vertical-end';
	import CheckIcon from '@lucide/svelte/icons/check';
	import { useQueryClient } from '@tanstack/svelte-query';
	import { sessionStore } from '$lib/stores/session';
	import { createListOrganizations, createCreateOrganization } from '$lib/api/endpoints/organizations/organizations';
	import { createSetActiveOrganization, getGetSessionQueryKey } from '$lib/api/endpoints/auth/auth';
	
	// Props: accept teams but ignore them to avoid breakage
	let { teams }: { teams?: any } = $props();

	const sidebar = useSidebar();
	const queryClient = useQueryClient();
	
	// Active Org
	const activeOrg = $derived($sessionStore?.active_organization);

	// List Organizations
	const orgsQuery = createListOrganizations();
	const organizations = $derived.by(() => {
		const res = orgsQuery.data;
		if (res?.status === 200) {
			return res.data.organizations ?? [];
		}
		return [];
	});

	// Mutations
	const setActiveMutation = createSetActiveOrganization();
	const createOrgMutation = createCreateOrganization();
	let setActiveError = $state('');
	let createOrgError = $state('');

	function handleSetActive(orgId: string) {
		setActiveError = '';
		setActiveMutation.mutate({ data: { organization_id: orgId } }, {
			onSuccess: () => {
				queryClient.invalidateQueries({ queryKey: getGetSessionQueryKey() });
			},
			onError: () => {
				setActiveError = 'Failed to switch organization';
			}
		});
	}

	// Create Org Dialog
	let dialogOpen = $state(false);
	let newOrgName = $state('');

	function handleCreateOrg() {
		if (!newOrgName.trim()) return;
		createOrgError = '';
		createOrgMutation.mutate({
				data: { 
					name: newOrgName.trim(), 
					set_as_active_organization: true 
				} 
		}, {
			onSuccess: (res) => {
				if (res.status !== 200) {
					createOrgError = (res.data as any)?.detail ?? 'Failed to create organization';
					return;
				}
				dialogOpen = false;
				newOrgName = '';
				queryClient.invalidateQueries({ queryKey: getGetSessionQueryKey() });
				orgsQuery.refetch(); 
			},
			onError: () => {
				createOrgError = 'Failed to create organization';
			}
		});
	}

	function closeDialog() {
		dialogOpen = false;
		newOrgName = '';
		createOrgError = '';
	}
</script>

<Sidebar.Menu>
  <Sidebar.MenuItem>
    <DropdownMenu.Root>
      <DropdownMenu.Trigger>
        {#snippet child({ props })}
          <Sidebar.MenuButton
            {...props}
            size="lg"
            class="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
          >
            <div
              class="bg-sidebar-primary text-sidebar-primary-foreground flex aspect-square size-8 items-center justify-center rounded-lg"
            >
              <GalleryVerticalEndIcon class="size-4" />
            </div>
            <div class="grid flex-1 text-start text-sm leading-tight">
              <span class="truncate font-medium">
                {activeOrg?.name ?? 'Select Organization'}
              </span>
              <span class="truncate text-xs">Free</span>
            </div>
            <ChevronsUpDownIcon class="ms-auto" />
          </Sidebar.MenuButton>
        {/snippet}
      </DropdownMenu.Trigger>
      <DropdownMenu.Content
        class="w-(--bits-dropdown-menu-anchor-width) min-w-56 rounded-lg"
        align="start"
        side={sidebar.isMobile ? "bottom" : "right"}
        sideOffset={4}
      >
        <DropdownMenu.Label class="text-muted-foreground text-xs">Organizations</DropdownMenu.Label>
        {#each organizations as org (org.id)}
          <DropdownMenu.Item 
            onSelect={() => handleSetActive(org.id)} 
            class="gap-2 p-2"
          >
            <div class="flex size-6 items-center justify-center rounded-md border">
              <GalleryVerticalEndIcon class="size-3.5 shrink-0" />
            </div>
            {org.name}
            {#if activeOrg?.id === org.id}
                <CheckIcon class="ml-auto size-4" />
            {/if}
          </DropdownMenu.Item>
        {/each}
        <DropdownMenu.Separator />
        <DropdownMenu.Item class="gap-2 p-2" onSelect={() => (dialogOpen = true)}>
          <div
            class="flex size-6 items-center justify-center rounded-md border bg-transparent"
          >
            <PlusIcon class="size-4" />
          </div>
          <div class="text-muted-foreground font-medium">Create Organization</div>
        </DropdownMenu.Item>
      </DropdownMenu.Content>
    </DropdownMenu.Root>
  </Sidebar.MenuItem>
</Sidebar.Menu>
{#if setActiveError}
	<div class="text-destructive px-2 pt-1 text-xs">{setActiveError}</div>
{/if}

<Dialog.Root bind:open={dialogOpen} onOpenChange={(open) => { if (!open) closeDialog(); }}>
    <Dialog.Content class="sm:max-w-md">
        <Dialog.Header>
            <Dialog.Title>Create Organization</Dialog.Title>
            <Dialog.Description>
                Add a new organization to your account.
            </Dialog.Description>
        </Dialog.Header>
        <div class="flex flex-col gap-4 py-4">
            <div class="flex flex-col gap-2">
                <label for="org-name" class="text-sm font-medium">Organization Name</label>
                <Input id="org-name" bind:value={newOrgName} placeholder="Acme Inc." />
            </div>
			{#if createOrgError}
				<div class="text-destructive bg-destructive/10 rounded-md px-3 py-2 text-sm">
					{createOrgError}
				</div>
			{/if}
        </div>
        <Dialog.Footer>
            <Button variant="outline" onclick={closeDialog}>Cancel</Button>
            <Button 
                onclick={handleCreateOrg} 
                disabled={!newOrgName.trim() || createOrgMutation.isPending}
            >
                {#if createOrgMutation.isPending}
                    Creating...
                {:else}
                    Create Organization
                {/if}
            </Button>
        </Dialog.Footer>
    </Dialog.Content>
</Dialog.Root>
