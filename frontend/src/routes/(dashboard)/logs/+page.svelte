<script lang="ts">
	import PageContainer from '../_components/page-container/page-container.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import ListIcon from '@lucide/svelte/icons/list';
	import BookmarkIcon from '@lucide/svelte/icons/bookmark';
	import LogFilters from './_components/log-filters.svelte';
	import LogHistogram from './_components/log-histogram.svelte';
	import LogList from './_components/log-list.svelte';
	import {
		mockLogs,
		generateHistogramData,
		type LogLevel,
		type LogEntry
	} from './_components/mock-data';

	// State
	let searchQuery = $state('');
	let selectedService = $state<string | undefined>(undefined);
	let selectedLevel = $state<LogLevel | undefined>(undefined);
	let selectedTimeRange = $state('1h');
	let chartVisible = $state(true);
	let isLive = $state(false);

	// Generate histogram data
	const histogramData = generateHistogramData();

	// Filter logs based on current filters
	const filteredLogs = $derived.by(() => {
		let result = mockLogs;

		if (selectedService) {
			result = result.filter((log) => log.service === selectedService);
		}

		if (selectedLevel) {
			result = result.filter((log) => log.level === selectedLevel);
		}

		if (searchQuery) {
			const query = searchQuery.toLowerCase();
			result = result.filter(
				(log) =>
					log.message.toLowerCase().includes(query) ||
					log.service.toLowerCase().includes(query) ||
					log.host.toLowerCase().includes(query) ||
					(log.group && log.group.toLowerCase().includes(query))
			);
		}

		return result;
	});

	function handleSearch() {
		// In a real app, this would trigger an API call
		console.log('Searching:', { searchQuery, selectedService, selectedLevel, selectedTimeRange });
	}

	function saveView() {
		// In a real app, this would open a dialog to save the current view
		console.log('Save view clicked');
	}
</script>

<PageContainer breadcrumbs={[{ title: 'Logs', href: '/logs' }, { title: 'All Logs' }]}>
	<div class="flex h-full flex-col gap-4">
		<!-- Header -->
		<div class="flex items-start justify-between">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<ListIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Logging</h1>
					<p class="text-sm text-muted-foreground">Search, filter and debug your logs</p>
				</div>
			</div>
			<Button onclick={saveView}>
				<BookmarkIcon class="mr-2 size-4" />
				Save view
			</Button>
		</div>

		<Separator />

		<!-- Filters -->
		<LogFilters
			bind:searchQuery
			bind:selectedService
			bind:selectedLevel
			bind:selectedTimeRange
			onSearch={handleSearch}
		/>

		<!-- Histogram Chart -->
		<LogHistogram
			data={histogramData}
			visible={chartVisible}
			onToggleVisibility={() => (chartVisible = !chartVisible)}
		/>

		<!-- Log List -->
		<div class="min-h-0 flex-1">
			<LogList logs={filteredLogs} {isLive} onToggleLive={() => (isLive = !isLive)} />
		</div>
	</div>
</PageContainer>
