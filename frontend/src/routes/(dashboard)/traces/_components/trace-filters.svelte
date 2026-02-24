<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import SearchIcon from '@lucide/svelte/icons/search';
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import XIcon from '@lucide/svelte/icons/x';

	type StatusFilter = 'ok' | 'error' | 'unset';

	let {
		searchQuery = $bindable(''),
		selectedService = $bindable<string | undefined>(undefined),
		selectedStatus = $bindable<StatusFilter | undefined>(undefined),
		selectedTimeRange = $bindable('1h'),
		minDuration = $bindable<number | undefined>(undefined),
		maxDuration = $bindable<number | undefined>(undefined),
		services: serviceNames = [],
		onSearch = () => {}
	}: {
		searchQuery?: string;
		selectedService?: string;
		selectedStatus?: StatusFilter;
		selectedTimeRange?: string;
		minDuration?: number;
		maxDuration?: number;
		services?: string[];
		onSearch?: () => void;
	} = $props();

	const serviceOptions = $derived(serviceNames.map((s) => ({ value: s, label: s })));

	const statuses: { value: StatusFilter; label: string }[] = [
		{ value: 'ok', label: 'Ok' },
		{ value: 'error', label: 'Error' },
		{ value: 'unset', label: 'Unset' }
	];

	const timeRanges = [
		{ value: '15m', label: '15 minutes' },
		{ value: '30m', label: '30 minutes' },
		{ value: '1h', label: '1 hour' },
		{ value: '3h', label: '3 hours' },
		{ value: '6h', label: '6 hours' },
		{ value: '12h', label: '12 hours' },
		{ value: '24h', label: '24 hours' },
		{ value: '7d', label: '7 days' },
		{ value: '14d', label: '14 days' },
		{ value: '30d', label: '30 days' }
	];

	function clearFilters() {
		searchQuery = '';
		selectedService = undefined;
		selectedStatus = undefined;
		selectedTimeRange = '1h';
		minDuration = undefined;
		maxDuration = undefined;
	}

	const hasActiveFilters = $derived(
		searchQuery ||
			selectedService ||
			selectedStatus ||
			selectedTimeRange !== '1h' ||
			minDuration !== undefined ||
			maxDuration !== undefined
	);
</script>

<div class="flex flex-wrap items-center gap-2">
	<!-- Service Select -->
	<Select.Root type="single" bind:value={selectedService}>
		<Select.Trigger class="w-[140px]" size="sm">
			{#if selectedService}
				<span>{selectedService}</span>
			{:else}
				<span class="text-muted-foreground">Service</span>
			{/if}
		</Select.Trigger>
		<Select.Content>
			{#each serviceOptions as service (service.value)}
				<Select.Item value={service.value}>{service.label}</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>

	<!-- Status Select -->
	<Select.Root type="single" bind:value={selectedStatus}>
		<Select.Trigger class="w-[120px]" size="sm">
			{#if selectedStatus}
				<span class="capitalize">{selectedStatus}</span>
			{:else}
				<span class="text-muted-foreground">Status</span>
			{/if}
		</Select.Trigger>
		<Select.Content>
			{#each statuses as status (status.value)}
				<Select.Item value={status.value}>{status.label}</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>

	<!-- Duration inputs -->
	<Input
		type="number"
		placeholder="Min ms"
		class="h-8 w-[90px]"
		value={minDuration ?? ''}
		oninput={(e) => {
			const v = parseInt(e.currentTarget.value);
			minDuration = isNaN(v) ? undefined : v;
		}}
	/>
	<Input
		type="number"
		placeholder="Max ms"
		class="h-8 w-[90px]"
		value={maxDuration ?? ''}
		oninput={(e) => {
			const v = parseInt(e.currentTarget.value);
			maxDuration = isNaN(v) ? undefined : v;
		}}
	/>

	<!-- Search Input -->
	<div class="relative min-w-[250px] flex-1">
		<SearchIcon class="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
		<Input
			bind:value={searchQuery}
			placeholder={'Search span names...'}
			class="h-8 pl-9"
			onkeydown={(e) => e.key === 'Enter' && onSearch()}
		/>
	</div>

	<!-- Time Range Select -->
	<Select.Root type="single" bind:value={selectedTimeRange}>
		<Select.Trigger class="w-[100px]" size="sm">
			<CalendarIcon class="size-4" />
			<span
				>{timeRanges.find((t) => t.value === selectedTimeRange)?.label.split(' ')[0] || '1H'}</span
			>
		</Select.Trigger>
		<Select.Content>
			{#each timeRanges as range (range.value)}
				<Select.Item value={range.value}>{range.label}</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>

	<!-- Clear Filters -->
	{#if hasActiveFilters}
		<Button variant="ghost" size="sm" onclick={clearFilters}>
			<XIcon class="mr-1 size-4" />
			Clear
		</Button>
	{/if}
</div>
