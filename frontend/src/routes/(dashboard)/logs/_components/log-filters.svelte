<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import * as Popover from '$lib/components/ui/popover/index.js';
	import SearchIcon from '@lucide/svelte/icons/search';
	import CalendarIcon from '@lucide/svelte/icons/calendar';
	import XIcon from '@lucide/svelte/icons/x';

	type LogLevel = 'debug' | 'info' | 'warn' | 'error' | 'fatal';

	let {
		searchQuery = $bindable(''),
		selectedService = $bindable<string | undefined>(undefined),
		selectedLevel = $bindable<LogLevel | undefined>(undefined),
		selectedTimeRange = $bindable('1h'),
		services: serviceNames = [],
		onSearch = () => {}
	}: {
		searchQuery?: string;
		selectedService?: string;
		selectedLevel?: LogLevel;
		selectedTimeRange?: string;
		services?: string[];
		onSearch?: () => void;
	} = $props();

	const serviceOptions = $derived(serviceNames.map((s) => ({ value: s, label: s })));

	const levels: { value: LogLevel; label: string }[] = [
		{ value: 'debug', label: 'Debug' },
		{ value: 'info', label: 'Info' },
		{ value: 'warn', label: 'Warning' },
		{ value: 'error', label: 'Error' },
		{ value: 'fatal', label: 'Fatal' }
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
		selectedLevel = undefined;
		selectedTimeRange = '1h';
	}

	const hasActiveFilters = $derived(
		searchQuery || selectedService || selectedLevel || selectedTimeRange !== '1h'
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

	<!-- Severity Select -->
	<Select.Root type="single" bind:value={selectedLevel}>
		<Select.Trigger class="w-[120px]" size="sm">
			{#if selectedLevel}
				<span class="capitalize">{selectedLevel}</span>
			{:else}
				<span class="text-muted-foreground">Severity</span>
			{/if}
		</Select.Trigger>
		<Select.Content>
			{#each levels as level (level.value)}
				<Select.Item value={level.value}>{level.label}</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>

	<!-- Search Input -->
	<div class="relative min-w-[300px] flex-1">
		<SearchIcon class="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
		<Input
			bind:value={searchQuery}
			placeholder={'hostname="frontend1" segfault'}
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

	<!-- Custom Range Button -->
	<Button variant="outline" size="sm">Custom range</Button>

	<!-- Clear Filters -->
	{#if hasActiveFilters}
		<Button variant="ghost" size="sm" onclick={clearFilters}>
			<XIcon class="mr-1 size-4" />
			Clear
		</Button>
	{/if}
</div>
