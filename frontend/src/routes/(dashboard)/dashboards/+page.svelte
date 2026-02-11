<script lang="ts">
	import PageContainer from '../_components/page-container/page-container.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import LayoutDashboardIcon from '@lucide/svelte/icons/layout-dashboard';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import SearchIcon from '@lucide/svelte/icons/search';
	import TrashIcon from '@lucide/svelte/icons/trash-2';
	import { goto } from '$app/navigation';
	import {
		createListDashboards,
		createCreateDashboard,
		createDeleteDashboard,
		getListDashboardsQueryKey
	} from '$lib/api/endpoints/dashboards/dashboards';
	import { sessionStore } from '$lib/stores/session';
	import { useQueryClient } from '@tanstack/svelte-query';
	import type { Dashboard } from '$lib/api/model';

	let searchQuery = $state('');
	let createOpen = $state(false);
	let newName = $state('');
	let newDescription = $state('');

	const orgId = $derived($sessionStore?.active_organization?.id ?? '');
	const queryClient = useQueryClient();

	const dashboardsQuery = createListDashboards(() => orgId);

	const dashboards = $derived.by((): Dashboard[] => {
		const resp = dashboardsQuery.data;
		if (resp && resp.status === 200) {
			return resp.data.dashboards ?? [];
		}
		return [];
	});

	const filtered = $derived.by(() => {
		if (!searchQuery) return dashboards;
		const q = searchQuery.toLowerCase();
		return dashboards.filter(
			(d) =>
				d.name.toLowerCase().includes(q) || d.description.toLowerCase().includes(q)
		);
	});

	const createMutation = createCreateDashboard();
	const deleteMutation = createDeleteDashboard();

	async function handleCreate() {
		if (!newName.trim() || !orgId) return;
		const resp = await createMutation.mutateAsync({
			organizationId: orgId,
			data: {
				name: newName.trim(),
				description: newDescription.trim()
			}
		});
		if (resp.status === 200) {
			queryClient.invalidateQueries({ queryKey: getListDashboardsQueryKey(orgId) });
			createOpen = false;
			newName = '';
			newDescription = '';
			goto(`/dashboards/${resp.data.id}`);
		}
	}

	async function handleDelete(e: MouseEvent, dashboardId: string) {
		e.stopPropagation();
		if (!confirm('Delete this dashboard?')) return;
		await deleteMutation.mutateAsync({ organizationId: orgId, dashboardId });
		queryClient.invalidateQueries({ queryKey: getListDashboardsQueryKey(orgId) });
	}

	function formatDate(dateStr: string): string {
		const d = new Date(dateStr);
		return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
	}
</script>

<PageContainer breadcrumbs={[{ title: 'Dashboards', href: '/dashboards' }, { title: 'All Dashboards' }]}>
	<div class="flex h-full flex-col gap-4">
		<!-- Header -->
		<div class="flex items-start justify-between">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<LayoutDashboardIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Dashboards</h1>
					<p class="text-sm text-muted-foreground">Create and manage custom metric dashboards</p>
				</div>
			</div>
			<Dialog.Root bind:open={createOpen}>
				<Dialog.Trigger>
					{#snippet child({ props })}
						<Button {...props}>
							<PlusIcon class="mr-2 size-4" />
							New Dashboard
						</Button>
					{/snippet}
				</Dialog.Trigger>
				<Dialog.Content class="sm:max-w-md">
					<Dialog.Header>
						<Dialog.Title>Create Dashboard</Dialog.Title>
						<Dialog.Description>Give your new dashboard a name and optional description.</Dialog.Description>
					</Dialog.Header>
					<div class="flex flex-col gap-4 py-4">
						<div class="flex flex-col gap-2">
							<Label for="dash-name">Name</Label>
							<Input id="dash-name" placeholder="My Dashboard" bind:value={newName} />
						</div>
						<div class="flex flex-col gap-2">
							<Label for="dash-desc">Description</Label>
							<Input id="dash-desc" placeholder="Optional description..." bind:value={newDescription} />
						</div>
					</div>
					<Dialog.Footer>
						<Button variant="outline" onclick={() => (createOpen = false)}>Cancel</Button>
						<Button onclick={handleCreate} disabled={!newName.trim() || createMutation.isPending}>
							{createMutation.isPending ? 'Creating...' : 'Create'}
						</Button>
					</Dialog.Footer>
				</Dialog.Content>
			</Dialog.Root>
		</div>

		<Separator />

		<!-- Search -->
		<div class="relative max-w-sm">
			<SearchIcon class="absolute left-2.5 top-2.5 size-4 text-muted-foreground" />
			<Input placeholder="Search dashboards..." class="pl-9" bind:value={searchQuery} />
		</div>

		<!-- Dashboard grid -->
		{#if dashboardsQuery.isPending}
			<div class="flex flex-1 items-center justify-center">
				<p class="text-sm text-muted-foreground">Loading dashboards...</p>
			</div>
		{:else if filtered.length === 0}
			<div class="flex flex-1 flex-col items-center justify-center gap-3 rounded-lg border border-dashed p-12">
				<LayoutDashboardIcon class="size-10 text-muted-foreground/50" />
				<p class="text-sm text-muted-foreground">
					{searchQuery ? 'No dashboards match your search' : 'No dashboards yet. Create one to get started.'}
				</p>
				{#if !searchQuery}
					<Button variant="outline" onclick={() => (createOpen = true)}>
						<PlusIcon class="mr-2 size-4" />
						Create Dashboard
					</Button>
				{/if}
			</div>
		{:else}
			<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
				{#each filtered as dashboard (dashboard.id)}
					<button
						type="button"
						class="text-left"
						onclick={() => goto(`/dashboards/${dashboard.id}`)}
					>
						<Card.Root class="transition-colors hover:bg-accent/50">
							<Card.Header class="pb-2">
								<div class="flex items-start justify-between">
									<Card.Title class="text-base">{dashboard.name}</Card.Title>
									<Button
										variant="ghost"
										size="icon"
										class="size-7 shrink-0 text-muted-foreground hover:text-destructive"
										onclick={(e) => handleDelete(e, dashboard.id)}
									>
										<TrashIcon class="size-3.5" />
									</Button>
								</div>
								{#if dashboard.description}
									<Card.Description class="line-clamp-2">{dashboard.description}</Card.Description>
								{/if}
							</Card.Header>
							<Card.Content>
								<div class="flex items-center gap-3 text-xs text-muted-foreground">
									<span>{(dashboard.panels ?? []).length} panel{(dashboard.panels ?? []).length !== 1 ? 's' : ''}</span>
									<span>Updated {formatDate(dashboard.updated_at)}</span>
								</div>
							</Card.Content>
						</Card.Root>
					</button>
				{/each}
			</div>
		{/if}
	</div>
</PageContainer>
