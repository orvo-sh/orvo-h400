<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import * as Popover from '$lib/components/ui/popover/index.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import SearchIcon from '@lucide/svelte/icons/search';
	import XIcon from '@lucide/svelte/icons/x';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import SparklesIcon from '@lucide/svelte/icons/sparkles';
	import type { LogLevel } from './mock-data';

	export type FilterOperator =
		| 'IN'
		| 'NIN'
		| 'CONTAINS'
		| 'NCONTAINS'
		| 'GT'
		| 'GTE'
		| 'LT'
		| 'LTE';

	export interface QueryFilter {
		id: string;
		field: string;
		operator: FilterOperator;
		value: string;
	}

	let {
		filters = $bindable<QueryFilter[]>([]),
		searchQuery = $bindable(''),
		onRunQuery = () => {},
		onAddFilter = (field: string, value: string) => {}
	}: {
		filters?: QueryFilter[];
		searchQuery?: string;
		onRunQuery?: () => void;
		onAddFilter?: (field: string, value: string) => void;
	} = $props();

	const fieldSuggestions = [
		{ field: 'service', label: 'Service', icon: 'cube' },
		{ field: 'level', label: 'Severity', icon: 'alert' },
		{ field: 'host', label: 'Host', icon: 'server' },
		{ field: 'source', label: 'Source', icon: 'database' },
		{ field: 'group', label: 'Group', icon: 'folder' },
		{ field: 'message', label: 'Message', icon: 'text' }
	];

	const operators: { value: FilterOperator; label: string; desc: string }[] = [
		{ value: 'IN', label: 'IN', desc: 'equals' },
		{ value: 'NIN', label: 'NIN', desc: 'not equals' },
		{ value: 'CONTAINS', label: 'CONTAINS', desc: 'contains text' },
		{ value: 'NCONTAINS', label: 'NCONTAINS', desc: 'excludes text' },
		{ value: 'GT', label: '>', desc: 'greater than' },
		{ value: 'GTE', label: '>=', desc: 'greater or equal' },
		{ value: 'LT', label: '<', desc: 'less than' },
		{ value: 'LTE', label: '<=', desc: 'less or equal' }
	];

	let showAddFilter = $state(false);
	let newFilterField = $state('');
	let newFilterOperator = $state<FilterOperator>('IN');
	let newFilterValue = $state('');

	function addFilter() {
		if (!newFilterField || !newFilterValue) return;

		const newFilter: QueryFilter = {
			id: `filter-${Date.now()}`,
			field: newFilterField,
			operator: newFilterOperator,
			value: newFilterValue
		};

		filters = [...filters, newFilter];
		resetNewFilter();
	}

	function removeFilter(id: string) {
		filters = filters.filter((f) => f.id !== id);
	}

	function resetNewFilter() {
		newFilterField = '';
		newFilterOperator = 'IN';
		newFilterValue = '';
		showAddFilter = false;
	}

	function getOperatorLabel(op: FilterOperator): string {
		return operators.find((o) => o.value === op)?.label || op;
	}

	function formatFilterDisplay(filter: QueryFilter): string {
		return `${filter.field} ${getOperatorLabel(filter.operator)} "${filter.value}"`;
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			onRunQuery();
		}
	}

	const hasFilters = $derived(filters.length > 0 || searchQuery.length > 0);
</script>

<div class="space-y-3">
	<!-- Query bar with filters -->
	<div class="flex min-h-[48px] flex-wrap items-center gap-2 rounded-lg border bg-card p-2">
		<!-- Active filters as chips -->
		{#each filters as filter (filter.id)}
			<Badge variant="secondary" class="gap-1 py-1 pr-1 pl-2 font-mono text-xs">
				<span class="text-muted-foreground">{filter.field}</span>
				<span class="font-medium text-primary">{getOperatorLabel(filter.operator)}</span>
				<span>'{filter.value}'</span>
				<button
					class="ml-1 rounded-full p-0.5 transition-colors hover:bg-muted"
					onclick={() => removeFilter(filter.id)}
				>
					<XIcon class="size-3" />
				</button>
			</Badge>
		{/each}

		<!-- Add filter button/popover -->
		<Popover.Root bind:open={showAddFilter}>
			<Popover.Trigger>
				{#snippet child({ props })}
					<Button variant="ghost" size="sm" class="h-7 gap-1 text-muted-foreground" {...props}>
						<PlusIcon class="size-3" />
						Add filter
					</Button>
				{/snippet}
			</Popover.Trigger>
			<Popover.Content class="w-80 p-3" align="start">
				<div class="space-y-3">
					<div class="text-sm font-medium">Add Filter</div>

					<!-- Field selector -->
					<div class="space-y-1">
						<label class="text-xs text-muted-foreground" for="log-filter-field">Field</label>
						<DropdownMenu.Root>
							<DropdownMenu.Trigger class="w-full">
								{#snippet child({ props })}
									<Button id="log-filter-field" variant="outline" class="w-full justify-start" size="sm" {...props}>
										{newFilterField || 'Select field...'}
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content class="w-56">
								{#each fieldSuggestions as suggestion (suggestion.field)}
									<DropdownMenu.Item onclick={() => (newFilterField = suggestion.field)}>
										{suggestion.label}
										<span class="ml-auto text-xs text-muted-foreground">{suggestion.field}</span>
									</DropdownMenu.Item>
								{/each}
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					</div>

					<!-- Operator selector -->
					<div class="space-y-1">
						<label class="text-xs text-muted-foreground" for="log-filter-operator">Operator</label>
						<DropdownMenu.Root>
							<DropdownMenu.Trigger class="w-full">
								{#snippet child({ props })}
									<Button
										id="log-filter-operator"
										variant="outline"
										class="w-full justify-start font-mono"
										size="sm"
										{...props}
									>
										{getOperatorLabel(newFilterOperator)}
										<span class="ml-2 font-normal text-muted-foreground">
											({operators.find((o) => o.value === newFilterOperator)?.desc})
										</span>
									</Button>
								{/snippet}
							</DropdownMenu.Trigger>
							<DropdownMenu.Content class="w-56">
								{#each operators as op (op.value)}
									<DropdownMenu.Item onclick={() => (newFilterOperator = op.value)}>
										<span class="w-16 font-mono">{op.label}</span>
										<span class="text-muted-foreground">{op.desc}</span>
									</DropdownMenu.Item>
								{/each}
							</DropdownMenu.Content>
						</DropdownMenu.Root>
					</div>

					<!-- Value input -->
					<div class="space-y-1">
						<label class="text-xs text-muted-foreground" for="log-filter-value">Value</label>
						<Input
							id="log-filter-value"
							bind:value={newFilterValue}
							placeholder="Enter value..."
							class="h-8"
							onkeydown={(e: KeyboardEvent) => e.key === 'Enter' && addFilter()}
						/>
					</div>

					<div class="flex justify-end gap-2">
						<Button variant="ghost" size="sm" onclick={resetNewFilter}>Cancel</Button>
						<Button size="sm" onclick={addFilter} disabled={!newFilterField || !newFilterValue}>
							Add
						</Button>
					</div>
				</div>
			</Popover.Content>
		</Popover.Root>

		<!-- Fulltext search -->
		<div class="min-w-[200px] flex-1">
			<div class="relative">
				<SearchIcon class="absolute top-1/2 left-2 size-4 -translate-y-1/2 text-muted-foreground" />
				<Input
					bind:value={searchQuery}
					placeholder="Search logs... (try: 'failed' OR 'error')"
					class="h-8 border-0 bg-transparent pl-8 shadow-none focus-visible:ring-0"
					onkeydown={handleKeydown}
				/>
			</div>
		</div>

		<!-- Run query button -->
		<Button size="sm" onclick={onRunQuery} class="shrink-0">
			<SparklesIcon class="mr-1 size-4" />
			Run Query
		</Button>
	</div>

	<!-- Quick tip -->
	{#if !hasFilters}
		<p class="px-1 text-xs text-muted-foreground">
			Tip: Click on any value in the log details to quickly add it as a filter
		</p>
	{/if}
</div>
