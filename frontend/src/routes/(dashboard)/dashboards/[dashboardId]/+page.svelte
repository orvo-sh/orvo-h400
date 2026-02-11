<script lang="ts">
	import PageContainer from '../../_components/page-container/page-container.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import LayoutDashboardIcon from '@lucide/svelte/icons/layout-dashboard';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import PencilIcon from '@lucide/svelte/icons/pencil';
	import TrashIcon from '@lucide/svelte/icons/trash-2';
	import SaveIcon from '@lucide/svelte/icons/save';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import GripVerticalIcon from '@lucide/svelte/icons/grip-vertical';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import {
		createGetDashboard,
		createUpdateDashboard,
		createDeleteDashboard,
		getGetDashboardQueryKey
	} from '$lib/api/endpoints/dashboards/dashboards';
	import { createQueryTimeseries } from '$lib/api/endpoints/metrics/metrics';
	import { sessionStore } from '$lib/stores/session';
	import { useQueryClient } from '@tanstack/svelte-query';
	import type {
		Dashboard,
		DashboardPanel,
		DashboardLayout,
		DashboardPanelQuery,
		Timeseries
	} from '$lib/api/model';
	import MetricChart from '../../metrics/_components/metric-chart.svelte';

	const dashboardId = $derived(page.params.dashboardId ?? '');
	const orgId = $derived($sessionStore?.active_organization?.id ?? '');
	const queryClient = useQueryClient();

	// Fetch dashboard
	const dashboardQuery = createGetDashboard(
		() => orgId,
		() => dashboardId
	);

	const dashboard = $derived.by((): Dashboard | null => {
		const resp = dashboardQuery.data;
		if (resp && resp.status === 200) return resp.data;
		return null;
	});

	// Editable local state — synced from server on load
	let editName = $state('');
	let editDescription = $state('');
	let editPanels = $state<DashboardPanel[]>([]);
	let editLayout = $state<DashboardLayout[]>([]);
	let isDirty = $state(false);
	let initialized = $state(false);

	$effect(() => {
		if (dashboard && !initialized) {
			editName = dashboard.name;
			editDescription = dashboard.description;
			editPanels = structuredClone(dashboard.panels ?? []);
			editLayout = structuredClone(dashboard.layout ?? []);
			initialized = true;
		}
	});

	function markDirty() {
		isDirty = true;
	}

	// Time range state
	let selectedTimeRange = $state('1h');
	const timeRangeMs: Record<string, number> = {
		'15m': 15 * 60 * 1000,
		'30m': 30 * 60 * 1000,
		'1h': 60 * 60 * 1000,
		'3h': 3 * 60 * 60 * 1000,
		'6h': 6 * 60 * 60 * 1000,
		'12h': 12 * 60 * 60 * 1000,
		'24h': 24 * 60 * 60 * 1000,
		'7d': 7 * 24 * 60 * 60 * 1000
	};

	const defaultSteps: Record<string, string> = {
		'15m': '1m',
		'30m': '1m',
		'1h': '1m',
		'3h': '5m',
		'6h': '5m',
		'12h': '15m',
		'24h': '30m',
		'7d': '1h'
	};

	function computeTimeRange(range: string): { start: string; end: string } {
		const now = new Date();
		const ms = timeRangeMs[range] ?? 3600000;
		return {
			start: new Date(now.getTime() - ms).toISOString(),
			end: now.toISOString()
		};
	}

	const initialRange = computeTimeRange('1h');
	let queryStart = $state(initialRange.start);
	let queryEnd = $state(initialRange.end);

	function refreshTime() {
		const range = computeTimeRange(selectedTimeRange);
		queryStart = range.start;
		queryEnd = range.end;
	}

	$effect(() => {
		selectedTimeRange;
		refreshTime();
	});

	// Panel add dialog
	let addPanelOpen = $state(false);
	let panelTitle = $state('');
	let panelMetric = $state('');
	let panelAggregation = $state('avg');
	let panelStep = $state('');

	// Edit panel dialog
	let editPanelOpen = $state(false);
	let editPanelIndex = $state(-1);
	let editPanelTitle = $state('');
	let editPanelMetric = $state('');
	let editPanelAggregation = $state('avg');
	let editPanelStep = $state('');

	function generateId(): string {
		return 'panel_' + Math.random().toString(36).substring(2, 10);
	}

	function handleAddPanel() {
		if (!panelTitle.trim() || !panelMetric.trim()) return;
		const id = generateId();
		const newPanel: DashboardPanel = {
			id,
			title: panelTitle.trim(),
			type: 'timeseries',
			query: {
				metric_name: panelMetric.trim(),
				aggregation: panelAggregation,
				step: panelStep || undefined
			}
		};
		// Calculate layout position: stack panels in rows of 2
		const maxY = editLayout.reduce((max, l) => Math.max(max, l.y + l.h), 0);
		const panelsInLastRow = editLayout.filter((l) => l.y + l.h === maxY);
		let newLayout: DashboardLayout;
		if (panelsInLastRow.length < 2 && editLayout.length > 0) {
			const lastPanel = panelsInLastRow[panelsInLastRow.length - 1];
			newLayout = { panel_id: id, x: lastPanel ? lastPanel.x + lastPanel.w : 0, y: maxY > 0 ? maxY - 4 : 0, w: 6, h: 4 };
		} else {
			newLayout = { panel_id: id, x: 0, y: maxY, w: 6, h: 4 };
		}

		editPanels = [...editPanels, newPanel];
		editLayout = [...editLayout, newLayout];
		addPanelOpen = false;
		panelTitle = '';
		panelMetric = '';
		panelAggregation = 'avg';
		panelStep = '';
		markDirty();
	}

	function openEditPanel(index: number) {
		const panel = editPanels[index];
		if (!panel) return;
		editPanelIndex = index;
		editPanelTitle = panel.title;
		editPanelMetric = panel.query.metric_name;
		editPanelAggregation = panel.query.aggregation;
		editPanelStep = panel.query.step ?? '';
		editPanelOpen = true;
	}

	function handleEditPanel() {
		if (editPanelIndex < 0 || !editPanelTitle.trim() || !editPanelMetric.trim()) return;
		editPanels = editPanels.map((p, i) => {
			if (i !== editPanelIndex) return p;
			return {
				...p,
				title: editPanelTitle.trim(),
				query: {
					...p.query,
					metric_name: editPanelMetric.trim(),
					aggregation: editPanelAggregation,
					step: editPanelStep || undefined
				}
			};
		});
		editPanelOpen = false;
		markDirty();
	}

	function removePanel(index: number) {
		const panel = editPanels[index];
		if (!panel) return;
		editPanels = editPanels.filter((_, i) => i !== index);
		editLayout = editLayout.filter((l) => l.panel_id !== panel.id);
		markDirty();
	}

	// Save
	const updateMutation = createUpdateDashboard();
	const deleteMutation = createDeleteDashboard();

	async function handleSave() {
		if (!orgId || !dashboardId) return;
		await updateMutation.mutateAsync({
			organizationId: orgId,
			dashboardId,
			data: {
				name: editName,
				description: editDescription,
				panels: editPanels,
				layout: editLayout
			}
		});
		isDirty = false;
		queryClient.invalidateQueries({ queryKey: getGetDashboardQueryKey(orgId, dashboardId) });
	}

	async function handleDeleteDashboard() {
		if (!confirm('Delete this dashboard? This cannot be undone.')) return;
		await deleteMutation.mutateAsync({ organizationId: orgId, dashboardId });
		goto('/dashboards');
	}

	// Panel data queries — each panel gets its own timeseries query
	function getPanelStep(panel: DashboardPanel): string {
		return panel.query.step || defaultSteps[selectedTimeRange] || '1m';
	}
</script>

<!-- Panel renderer component (inline snippet) -->
{#snippet panelRenderer(panel: DashboardPanel, index: number)}
	{@const panelQuery = createQueryTimeseries(
		() => orgId,
		() => ({
			metric: panel.query.metric_name,
			start: queryStart,
			end: queryEnd,
			step: getPanelStep(panel),
			aggregation: panel.query.aggregation || 'avg',
			service: panel.query.filters?.['service.name'] || undefined,
			group_by: panel.query.group_by?.join(',') || undefined
		}),
		() => ({
			query: {
				enabled: !!orgId && !!panel.query.metric_name
			}
		})
	)}
	{@const series = (() => {
		const resp = panelQuery.data;
		if (resp && resp.status === 200) return resp.data.series ?? [];
		return [] as Timeseries[];
	})()}

	<Card.Root class="flex h-full flex-col">
		<Card.Header class="flex-row items-center justify-between space-y-0 pb-2">
			<Card.Title class="text-sm font-medium">{panel.title}</Card.Title>
			<div class="flex items-center gap-1">
				<Button
					variant="ghost"
					size="icon"
					class="size-6 text-muted-foreground"
					onclick={() => openEditPanel(index)}
				>
					<PencilIcon class="size-3" />
				</Button>
				<Button
					variant="ghost"
					size="icon"
					class="size-6 text-muted-foreground hover:text-destructive"
					onclick={() => removePanel(index)}
				>
					<TrashIcon class="size-3" />
				</Button>
			</div>
		</Card.Header>
		<Card.Content class="flex-1 pb-4">
			<MetricChart
				{series}
				metricName={panel.query.metric_name}
				loading={panelQuery.isPending}
			/>
		</Card.Content>
	</Card.Root>
{/snippet}

<PageContainer
	breadcrumbs={[
		{ title: 'Dashboards', href: '/dashboards' },
		{ title: dashboard?.name ?? 'Loading...' }
	]}
>
	<div class="flex h-full flex-col gap-4">
		{#if dashboardQuery.isPending}
			<div class="flex flex-1 items-center justify-center">
				<p class="text-sm text-muted-foreground">Loading dashboard...</p>
			</div>
		{:else if !dashboard}
			<div class="flex flex-1 flex-col items-center justify-center gap-3">
				<p class="text-sm text-muted-foreground">Dashboard not found</p>
				<Button variant="outline" onclick={() => goto('/dashboards')}>Back to Dashboards</Button>
			</div>
		{:else}
			<!-- Header -->
			<div class="flex items-start justify-between">
				<div class="flex items-center gap-3">
					<div class="rounded-lg bg-primary/10 p-2">
						<LayoutDashboardIcon class="size-6 text-primary" />
					</div>
					<div class="flex flex-col gap-1">
						<Input
							class="h-8 border-none bg-transparent p-0 text-xl font-semibold shadow-none focus-visible:ring-0"
							bind:value={editName}
							oninput={markDirty}
						/>
						<Input
							class="h-6 border-none bg-transparent p-0 text-sm text-muted-foreground shadow-none focus-visible:ring-0"
							placeholder="Add a description..."
							bind:value={editDescription}
							oninput={markDirty}
						/>
					</div>
				</div>
				<div class="flex items-center gap-2">
					<Select.Root
						type="single"
						value={selectedTimeRange}
						onValueChange={(v) => { if (v) selectedTimeRange = v; }}
					>
						<Select.Trigger class="w-28">
							{selectedTimeRange}
						</Select.Trigger>
						<Select.Content>
							{#each ['15m', '30m', '1h', '3h', '6h', '12h', '24h', '7d'] as range}
								<Select.Item value={range}>{range}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>

					<Button variant="outline" size="icon" onclick={refreshTime}>
						<RefreshCwIcon class="size-4" />
					</Button>

					{#if isDirty}
						<Button onclick={handleSave} disabled={updateMutation.isPending}>
							<SaveIcon class="mr-2 size-4" />
							{updateMutation.isPending ? 'Saving...' : 'Save'}
						</Button>
					{/if}

					<Button variant="destructive" size="icon" onclick={handleDeleteDashboard}>
						<TrashIcon class="size-4" />
					</Button>
				</div>
			</div>

			<Separator />

			<!-- Panels grid -->
			{#if editPanels.length === 0}
				<div class="flex flex-1 flex-col items-center justify-center gap-3 rounded-lg border border-dashed p-12">
					<GripVerticalIcon class="size-10 text-muted-foreground/50" />
					<p class="text-sm text-muted-foreground">No panels yet. Add a panel to start visualizing metrics.</p>
					<Button variant="outline" onclick={() => (addPanelOpen = true)}>
						<PlusIcon class="mr-2 size-4" />
						Add Panel
					</Button>
				</div>
			{:else}
				<div class="grid gap-4 sm:grid-cols-1 lg:grid-cols-2">
					{#each editPanels as panel, index (panel.id)}
						<div class="min-h-[320px]">
							{@render panelRenderer(panel, index)}
						</div>
					{/each}
				</div>

				<div class="flex justify-center pb-4">
					<Button variant="outline" onclick={() => (addPanelOpen = true)}>
						<PlusIcon class="mr-2 size-4" />
						Add Panel
					</Button>
				</div>
			{/if}
		{/if}
	</div>

	<!-- Add Panel Dialog -->
	<Dialog.Root bind:open={addPanelOpen}>
		<Dialog.Content class="sm:max-w-md">
			<Dialog.Header>
				<Dialog.Title>Add Panel</Dialog.Title>
				<Dialog.Description>Configure a new metric panel for this dashboard.</Dialog.Description>
			</Dialog.Header>
			<div class="flex flex-col gap-4 py-4">
				<div class="flex flex-col gap-2">
					<Label for="panel-title">Title</Label>
					<Input id="panel-title" placeholder="Request Rate" bind:value={panelTitle} />
				</div>
				<div class="flex flex-col gap-2">
					<Label for="panel-metric">Metric Name</Label>
					<Input id="panel-metric" placeholder="spans.request.count" bind:value={panelMetric} />
				</div>
				<div class="flex flex-col gap-2">
					<Label>Aggregation</Label>
					<Select.Root
						type="single"
						value={panelAggregation}
						onValueChange={(v) => { if (v) panelAggregation = v; }}
					>
						<Select.Trigger>{panelAggregation}</Select.Trigger>
						<Select.Content>
							{#each ['avg', 'sum', 'min', 'max', 'rate', 'count', 'last', 'p50', 'p90', 'p95', 'p99'] as agg}
								<Select.Item value={agg}>{agg}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
				<div class="flex flex-col gap-2">
					<Label for="panel-step">Step (optional)</Label>
					<Input id="panel-step" placeholder="Auto" bind:value={panelStep} />
				</div>
			</div>
			<Dialog.Footer>
				<Button variant="outline" onclick={() => (addPanelOpen = false)}>Cancel</Button>
				<Button onclick={handleAddPanel} disabled={!panelTitle.trim() || !panelMetric.trim()}>
					Add Panel
				</Button>
			</Dialog.Footer>
		</Dialog.Content>
	</Dialog.Root>

	<!-- Edit Panel Dialog -->
	<Dialog.Root bind:open={editPanelOpen}>
		<Dialog.Content class="sm:max-w-md">
			<Dialog.Header>
				<Dialog.Title>Edit Panel</Dialog.Title>
				<Dialog.Description>Update the panel configuration.</Dialog.Description>
			</Dialog.Header>
			<div class="flex flex-col gap-4 py-4">
				<div class="flex flex-col gap-2">
					<Label for="edit-panel-title">Title</Label>
					<Input id="edit-panel-title" bind:value={editPanelTitle} />
				</div>
				<div class="flex flex-col gap-2">
					<Label for="edit-panel-metric">Metric Name</Label>
					<Input id="edit-panel-metric" bind:value={editPanelMetric} />
				</div>
				<div class="flex flex-col gap-2">
					<Label>Aggregation</Label>
					<Select.Root
						type="single"
						value={editPanelAggregation}
						onValueChange={(v) => { if (v) editPanelAggregation = v; }}
					>
						<Select.Trigger>{editPanelAggregation}</Select.Trigger>
						<Select.Content>
							{#each ['avg', 'sum', 'min', 'max', 'rate', 'count', 'last', 'p50', 'p90', 'p95', 'p99'] as agg}
								<Select.Item value={agg}>{agg}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
				<div class="flex flex-col gap-2">
					<Label for="edit-panel-step">Step (optional)</Label>
					<Input id="edit-panel-step" bind:value={editPanelStep} />
				</div>
			</div>
			<Dialog.Footer>
				<Button variant="outline" onclick={() => (editPanelOpen = false)}>Cancel</Button>
				<Button onclick={handleEditPanel} disabled={!editPanelTitle.trim() || !editPanelMetric.trim()}>
					Save Changes
				</Button>
			</Dialog.Footer>
		</Dialog.Content>
	</Dialog.Root>
</PageContainer>
