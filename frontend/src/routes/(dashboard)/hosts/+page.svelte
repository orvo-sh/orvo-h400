<script lang="ts">
	import PageContainer from '../_components/page-container/page-container.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import MetricChart from '../metrics/_components/metric-chart.svelte';
	import { createQueryTimeseries } from '$lib/api/endpoints/metrics/metrics';
	import { sessionStore } from '$lib/stores/session';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import ServerIcon from '@lucide/svelte/icons/server';
	import CpuIcon from '@lucide/svelte/icons/cpu';
	import MemoryStickIcon from '@lucide/svelte/icons/memory-stick';
	import Clock3Icon from '@lucide/svelte/icons/clock-3';
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import HardDriveIcon from '@lucide/svelte/icons/hard-drive';
	import NetworkIcon from '@lucide/svelte/icons/network';
	import type { Timeseries, TimeseriesPoint } from '$lib/api/model';

	type HostSnapshot = {
		name: string;
		cpuUtilization: number | null;
		memoryUtilization: number | null;
		uptimeSeconds: number | null;
		lastSeen: string | null;
	};

	type SeriesDescriptor = {
		title: string;
		series: Timeseries[];
		loading: boolean;
	};

	const hostObserverServiceName = 'orvo-host-observer';
	const lookbackMs = 60 * 60 * 1000;
	const staleThresholdMs = 3 * 60 * 1000;
	const orgId = $derived($sessionStore?.active_organization?.id ?? '');

	function buildWindow() {
		const now = new Date();
		return {
			start: new Date(now.getTime() - lookbackMs).toISOString(),
			end: now.toISOString()
		};
	}

	let queryWindow = $state(buildWindow());
	let selectedHostName = $state<string | null>(null);

	function refreshWindow() {
		queryWindow = buildWindow();
	}

	function createHostQuery(metric: string, aggregation: string, groupBy?: string) {
		return createQueryTimeseries(
			() => orgId,
			() => ({
				metric,
				service: hostObserverServiceName,
				start: queryWindow.start,
				end: queryWindow.end,
				step: '1m',
				aggregation,
				group_by: groupBy,
				filters: selectedHostName ? `host.name=${selectedHostName}` : undefined
			}),
			() => ({
				query: {
					enabled: !!orgId && !!selectedHostName,
					refetchInterval: 30_000
				}
			})
		);
	}

	const cpuQuery = createQueryTimeseries(
		() => orgId,
		() => ({
			metric: 'system.cpu.utilization',
			service: hostObserverServiceName,
			start: queryWindow.start,
			end: queryWindow.end,
			step: '1m',
			aggregation: 'avg',
			group_by: 'host.name,state'
		}),
		() => ({
			query: {
				enabled: !!orgId,
				refetchInterval: 30_000
			}
		})
	);

	const memoryQuery = createQueryTimeseries(
		() => orgId,
		() => ({
			metric: 'system.memory.utilization',
			service: hostObserverServiceName,
			start: queryWindow.start,
			end: queryWindow.end,
			step: '1m',
			aggregation: 'avg',
			group_by: 'host.name'
		}),
		() => ({
			query: {
				enabled: !!orgId,
				refetchInterval: 30_000
			}
		})
	);

	const uptimeQuery = createQueryTimeseries(
		() => orgId,
		() => ({
			metric: 'system.uptime',
			service: hostObserverServiceName,
			start: queryWindow.start,
			end: queryWindow.end,
			step: '1m',
			aggregation: 'last',
			group_by: 'host.name'
		}),
		() => ({
			query: {
				enabled: !!orgId,
				refetchInterval: 30_000
			}
		})
	);

	const hostCpuQuery = createHostQuery('system.cpu.utilization', 'avg', 'state');
	const hostMemoryQuery = createHostQuery('system.memory.utilization', 'avg');
	const hostUptimeQuery = createHostQuery('system.uptime', 'last');
	const hostFilesystemQuery = createHostQuery('system.filesystem.utilization', 'avg', 'device,mountpoint');
	const hostNetworkQuery = createHostQuery('system.network.io', 'rate', 'device,direction');
	const hostPagingQuery = createHostQuery('system.paging.operations', 'rate', 'type');
	const hostLoad1Query = createHostQuery('system.cpu.load_average.1m', 'avg');
	const hostLoad5Query = createHostQuery('system.cpu.load_average.5m', 'avg');
	const hostLoad15Query = createHostQuery('system.cpu.load_average.15m', 'avg');

	const cpuSeries = $derived.by((): Timeseries[] => {
		const response = cpuQuery.data;
		if (response && response.status === 200) {
			return response.data.series ?? [];
		}
		return [];
	});

	const memorySeries = $derived.by((): Timeseries[] => {
		const response = memoryQuery.data;
		if (response && response.status === 200) {
			return response.data.series ?? [];
		}
		return [];
	});

	const uptimeSeries = $derived.by((): Timeseries[] => {
		const response = uptimeQuery.data;
		if (response && response.status === 200) {
			return response.data.series ?? [];
		}
		return [];
	});

	function latestPoint(points: TimeseriesPoint[] | null | undefined): TimeseriesPoint | null {
		if (!points || points.length === 0) return null;
		return points[points.length - 1];
	}

	function getSeries(query: { data: unknown }): Timeseries[] {
		const response = query.data as { status?: number; data?: { series?: Timeseries[] | null } } | undefined;
		if (response?.status === 200) {
			return response.data?.series ?? [];
		}
		return [];
	}

	function getHostName(labels: Record<string, string> | undefined): string {
		return labels?.['host.name']?.trim() || 'unknown-host';
	}

	const hosts = $derived.by((): HostSnapshot[] => {
		const snapshots = new Map<string, HostSnapshot>();

		for (const series of cpuSeries) {
			const hostName = getHostName(series.labels);
			const state = series.labels?.state?.toLowerCase() ?? '';
			if (state === 'idle') continue;

			const point = latestPoint(series.points);
			if (!point) continue;

			const existing = snapshots.get(hostName) ?? {
				name: hostName,
				cpuUtilization: 0,
				memoryUtilization: null,
				uptimeSeconds: null,
				lastSeen: null
			};
			existing.cpuUtilization = (existing.cpuUtilization ?? 0) + point.value;
			existing.lastSeen = maxTimestamp(existing.lastSeen, point.time);
			snapshots.set(hostName, existing);
		}

		for (const series of memorySeries) {
			const hostName = getHostName(series.labels);
			const point = latestPoint(series.points);
			if (!point) continue;

			const existing = snapshots.get(hostName) ?? {
				name: hostName,
				cpuUtilization: null,
				memoryUtilization: null,
				uptimeSeconds: null,
				lastSeen: null
			};
			existing.memoryUtilization = point.value;
			existing.lastSeen = maxTimestamp(existing.lastSeen, point.time);
			snapshots.set(hostName, existing);
		}

		for (const series of uptimeSeries) {
			const hostName = getHostName(series.labels);
			const point = latestPoint(series.points);
			if (!point) continue;

			const existing = snapshots.get(hostName) ?? {
				name: hostName,
				cpuUtilization: null,
				memoryUtilization: null,
				uptimeSeconds: null,
				lastSeen: null
			};
			existing.uptimeSeconds = point.value;
			existing.lastSeen = maxTimestamp(existing.lastSeen, point.time);
			snapshots.set(hostName, existing);
		}

		return Array.from(snapshots.values())
			.map((snapshot) => ({
				...snapshot,
				cpuUtilization:
					snapshot.cpuUtilization === null
						? null
						: Math.max(0, Math.min(snapshot.cpuUtilization, 1))
			}))
			.sort((left, right) => left.name.localeCompare(right.name));
	});

	$effect(() => {
		if (hosts.length === 0) {
			selectedHostName = null;
			return;
		}

		if (!selectedHostName || !hosts.some((host) => host.name === selectedHostName)) {
			selectedHostName = hosts[0].name;
		}
	});

	const selectedHost = $derived(hosts.find((host) => host.name === selectedHostName) ?? null);

	const onlineHosts = $derived(
		hosts.filter((host) => {
			if (!host.lastSeen) return false;
			return Date.now() - new Date(host.lastSeen).getTime() < staleThresholdMs;
		}).length
	);

	const averageCpu = $derived.by(() => {
		const values = hosts
			.map((host) => host.cpuUtilization)
			.filter((value): value is number => value !== null);
		if (values.length === 0) return null;
		return values.reduce((sum, value) => sum + value, 0) / values.length;
	});

	const peakMemory = $derived.by(() => {
		const values = hosts
			.map((host) => host.memoryUtilization)
			.filter((value): value is number => value !== null);
		if (values.length === 0) return null;
		return Math.max(...values);
	});

	function maxTimestamp(current: string | null, next: string): string {
		if (!current) return next;
		return new Date(next).getTime() > new Date(current).getTime() ? next : current;
	}

	function formatPercent(value: number | null): string {
		if (value === null) return 'Pending';
		return `${(value * 100).toFixed(value >= 0.1 ? 0 : 1)}%`;
	}

	function formatUptime(value: number | null): string {
		if (value === null) return 'Pending';
		const totalSeconds = Math.max(0, Math.floor(value));
		const days = Math.floor(totalSeconds / 86_400);
		const hours = Math.floor((totalSeconds % 86_400) / 3_600);
		const minutes = Math.floor((totalSeconds % 3_600) / 60);
		if (days > 0) return `${days}d ${hours}h`;
		if (hours > 0) return `${hours}h ${minutes}m`;
		return `${minutes}m`;
	}

	function formatLastSeen(timestamp: string | null): string {
		if (!timestamp) return 'No samples yet';
		return new Date(timestamp).toLocaleTimeString();
	}

	function formatAbsoluteTime(timestamp: string | null): string {
		if (!timestamp) return 'Waiting for samples';
		return new Date(timestamp).toLocaleString();
	}

	function hostStatusVariant(host: HostSnapshot): 'secondary' | 'outline' {
		if (!host.lastSeen) return 'outline';
		return Date.now() - new Date(host.lastSeen).getTime() < staleThresholdMs ? 'secondary' : 'outline';
	}

	function hostStatusLabel(host: HostSnapshot): string {
		if (!host.lastSeen) return 'Pending';
		return Date.now() - new Date(host.lastSeen).getTime() < staleThresholdMs ? 'Online' : 'Stale';
	}

	function renameSeries(series: Timeseries[], label: string): Timeseries[] {
		return series.map((item) => ({
			...item,
			labels: {
				...item.labels,
				series: label
			}
		}));
	}

	function aggregateCpuSeries(series: Timeseries[]): Timeseries[] {
		const totals = new Map<string, number>();
		for (const metricSeries of series) {
			if ((metricSeries.labels?.state ?? '').toLowerCase() === 'idle') continue;
			for (const point of metricSeries.points ?? []) {
				totals.set(point.time, (totals.get(point.time) ?? 0) + point.value);
			}
		}

		return [
			{
				labels: { metric: 'cpu', series: 'cpu-utilization' },
				points: Array.from(totals.entries())
					.sort(([left], [right]) => new Date(left).getTime() - new Date(right).getTime())
					.map(([time, value]) => ({
						time,
						value: Math.max(0, Math.min(value, 1))
					}))
			}
		];
	}

	function toSingleSeries(series: Timeseries[], label: string): Timeseries[] {
		const primary = series[0];
		if (!primary) return [];
		return [
			{
				labels: { metric: label, series: label },
				points: primary.points ?? []
			}
		];
	}

	function lastSeriesValue(series: Timeseries[]): number | null {
		const point = latestPoint(series[0]?.points);
		return point?.value ?? null;
	}

	const hostCpuDetailSeries = $derived.by(() => aggregateCpuSeries(getSeries(hostCpuQuery)));
	const hostMemoryDetailSeries = $derived.by(() => renameSeries(getSeries(hostMemoryQuery), 'memory'));
	const hostFilesystemDetailSeries = $derived.by(() => getSeries(hostFilesystemQuery));
	const hostNetworkDetailSeries = $derived.by(() => getSeries(hostNetworkQuery));
	const hostPagingDetailSeries = $derived.by(() => getSeries(hostPagingQuery));
	const hostLoadDetailSeries = $derived.by(() => [
		...renameSeries(getSeries(hostLoad1Query), 'load-1m'),
		...renameSeries(getSeries(hostLoad5Query), 'load-5m'),
		...renameSeries(getSeries(hostLoad15Query), 'load-15m')
	]);
	const hostUptimeDetailSeries = $derived.by(() => toSingleSeries(getSeries(hostUptimeQuery), 'uptime'));

	const hostDetailCards = $derived.by(
		(): { title: string; icon: typeof CpuIcon; value: string; detail: string }[] => [
			{
				title: 'CPU now',
				icon: CpuIcon,
				value: formatPercent(lastSeriesValue(hostCpuDetailSeries)),
				detail: 'Summed across non-idle CPU states.'
			},
			{
				title: 'Memory now',
				icon: MemoryStickIcon,
				value: formatPercent(lastSeriesValue(hostMemoryDetailSeries)),
				detail: 'Most recent host memory utilization.'
			},
			{
				title: 'Load average',
				icon: ActivityIcon,
				value: formatLoad(lastSeriesValue(renameSeries(getSeries(hostLoad1Query), 'load-1m'))),
				detail: 'One-minute load average.'
			},
			{
				title: 'Uptime',
				icon: Clock3Icon,
				value: formatUptime(lastSeriesValue(hostUptimeDetailSeries)),
				detail: 'Most recent system uptime sample.'
			}
		]
	);

	const hostCharts = $derived.by(
		(): SeriesDescriptor[] => [
			{
				title: 'CPU utilization',
				series: hostCpuDetailSeries,
				loading: hostCpuQuery.isPending
			},
			{
				title: 'Memory utilization',
				series: hostMemoryDetailSeries,
				loading: hostMemoryQuery.isPending
			},
			{
				title: 'Load averages',
				series: hostLoadDetailSeries,
				loading: hostLoad1Query.isPending || hostLoad5Query.isPending || hostLoad15Query.isPending
			},
			{
				title: 'Filesystem utilization',
				series: hostFilesystemDetailSeries,
				loading: hostFilesystemQuery.isPending
			},
			{
				title: 'Network IO rate',
				series: hostNetworkDetailSeries,
				loading: hostNetworkQuery.isPending
			},
			{
				title: 'Paging operations',
				series: hostPagingDetailSeries,
				loading: hostPagingQuery.isPending
			}
		]
	);

	function formatLoad(value: number | null): string {
		if (value === null) return 'Pending';
		return value.toFixed(2);
	}

	async function refreshAll() {
		refreshWindow();
		const work = [
			cpuQuery.refetch(),
			memoryQuery.refetch(),
			uptimeQuery.refetch()
		];
		if (selectedHostName) {
			work.push(
				hostCpuQuery.refetch(),
				hostMemoryQuery.refetch(),
				hostUptimeQuery.refetch(),
				hostFilesystemQuery.refetch(),
				hostNetworkQuery.refetch(),
				hostPagingQuery.refetch(),
				hostLoad1Query.refetch(),
				hostLoad5Query.refetch(),
				hostLoad15Query.refetch()
			);
		}
		await Promise.all(work);
	}
</script>

<PageContainer breadcrumbs={[{ title: 'Hosts', href: '/hosts' }, { title: 'Infrastructure' }]}>
	<div class="flex h-full flex-col gap-4">
		<div class="flex items-start justify-between gap-4">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<ServerIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Hosts</h1>
					<p class="text-sm text-muted-foreground">
						Inventory, drill-down charts, and live runtime telemetry from the deployment box.
					</p>
				</div>
			</div>
			<Button onclick={refreshAll}>
				<RefreshCwIcon class="mr-2 size-4" />
				Refresh
			</Button>
		</div>

		<Separator />

		<div class="grid gap-4 md:grid-cols-3">
			<Card.Root class="border-primary/20 bg-linear-to-br from-primary/10 via-background to-background">
				<Card.Content class="pt-6">
					<p class="text-sm font-medium text-muted-foreground">Hosts Reporting</p>
					<p class="mt-3 text-3xl font-semibold">{onlineHosts}</p>
					<p class="mt-2 text-sm text-muted-foreground">Active within the last three minutes.</p>
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Content class="pt-6">
					<div class="flex items-center gap-2 text-muted-foreground">
						<CpuIcon class="size-4" />
						<p class="text-sm font-medium">Average CPU</p>
					</div>
					<p class="mt-3 text-3xl font-semibold">{formatPercent(averageCpu)}</p>
					<p class="mt-2 text-sm text-muted-foreground">Summed from non-idle CPU states per host.</p>
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Content class="pt-6">
					<div class="flex items-center gap-2 text-muted-foreground">
						<MemoryStickIcon class="size-4" />
						<p class="text-sm font-medium">Peak Memory</p>
					</div>
					<p class="mt-3 text-3xl font-semibold">{formatPercent(peakMemory)}</p>
					<p class="mt-2 text-sm text-muted-foreground">Highest host memory utilization in the current window.</p>
				</Card.Content>
			</Card.Root>
		</div>

		<div class="grid gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
			<Card.Root class="overflow-hidden">
				<Card.Header class="border-b bg-muted/20">
					<div class="flex items-center justify-between gap-4">
						<div>
							<Card.Title class="text-base">Host Inventory</Card.Title>
							<Card.Description>
								Select a host to inspect live charts for CPU, load, filesystem pressure, and network traffic.
							</Card.Description>
						</div>
						<Badge variant="outline">{hosts.length} hosts</Badge>
					</div>
				</Card.Header>
				<Card.Content class="p-0">
					{#if cpuQuery.isPending && hosts.length === 0}
						<div class="p-6 text-sm text-muted-foreground">Waiting for host metrics...</div>
					{:else if hosts.length === 0}
						<div class="p-6 text-sm text-muted-foreground">
							No host telemetry has arrived yet. Once the collector is sending `system.*` metrics, the hosts view will populate automatically.
						</div>
					{:else}
						<div class="overflow-x-auto">
							<table class="w-full min-w-[760px] text-sm">
								<thead class="bg-muted/10 text-left text-xs uppercase tracking-[0.12em] text-muted-foreground">
									<tr>
										<th class="px-4 py-3 font-medium">Host</th>
										<th class="px-4 py-3 font-medium">Status</th>
										<th class="px-4 py-3 font-medium">CPU</th>
										<th class="px-4 py-3 font-medium">Memory</th>
										<th class="px-4 py-3 font-medium">Uptime</th>
										<th class="px-4 py-3 font-medium">Last Seen</th>
									</tr>
								</thead>
								<tbody>
									{#each hosts as host (host.name)}
										<tr
											class={`border-t transition-colors ${selectedHostName === host.name ? 'bg-primary/5' : 'hover:bg-muted/20'}`}
											onclick={() => {
												selectedHostName = host.name;
											}}
										>
											<td class="px-4 py-4">
												<div class="font-medium">{host.name}</div>
												<div class="mt-1 text-xs text-muted-foreground">service: {hostObserverServiceName}</div>
											</td>
											<td class="px-4 py-4">
												<Badge variant={hostStatusVariant(host)}>{hostStatusLabel(host)}</Badge>
											</td>
											<td class="px-4 py-4">
												<div class="flex items-center gap-2">
													<CpuIcon class="size-4 text-muted-foreground" />
													<span>{formatPercent(host.cpuUtilization)}</span>
												</div>
											</td>
											<td class="px-4 py-4">
												<div class="flex items-center gap-2">
													<MemoryStickIcon class="size-4 text-muted-foreground" />
													<span>{formatPercent(host.memoryUtilization)}</span>
												</div>
											</td>
											<td class="px-4 py-4">
												<div class="flex items-center gap-2">
													<Clock3Icon class="size-4 text-muted-foreground" />
													<span>{formatUptime(host.uptimeSeconds)}</span>
												</div>
											</td>
											<td class="px-4 py-4 text-muted-foreground">{formatLastSeen(host.lastSeen)}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{/if}
				</Card.Content>
			</Card.Root>

			<Card.Root>
				<Card.Header class="border-b bg-muted/20">
					<div class="flex items-center justify-between gap-3">
						<div>
							<Card.Title class="text-base">Selected Host</Card.Title>
							<Card.Description>
								Focused runtime view for the currently selected machine.
							</Card.Description>
						</div>
						{#if selectedHost}
							<Badge variant={hostStatusVariant(selectedHost)}>{hostStatusLabel(selectedHost)}</Badge>
						{/if}
					</div>
				</Card.Header>
				<Card.Content class="space-y-4 pt-6">
					{#if !selectedHost}
						<p class="text-sm text-muted-foreground">Select a host to inspect detailed telemetry.</p>
					{:else}
						<div class="space-y-2">
							<div class="flex items-center gap-3">
								<div class="rounded-lg bg-primary/10 p-2">
									<ServerIcon class="size-5 text-primary" />
								</div>
								<div>
									<p class="text-lg font-semibold">{selectedHost.name}</p>
									<p class="text-xs text-muted-foreground">
										Last seen {formatAbsoluteTime(selectedHost.lastSeen)}
									</p>
								</div>
							</div>
							<p class="text-sm text-muted-foreground">
								Live charts below are filtered to `host.name={selectedHost.name}` across the last hour.
							</p>
						</div>

						<div class="grid gap-3 sm:grid-cols-2">
							{#each hostDetailCards as card}
								<Card.Root class="bg-muted/15">
									<Card.Content class="pt-5">
										<div class="flex items-center gap-2 text-muted-foreground">
											<card.icon class="size-4" />
											<p class="text-sm font-medium">{card.title}</p>
										</div>
										<p class="mt-3 text-2xl font-semibold">{card.value}</p>
										<p class="mt-2 text-xs text-muted-foreground">{card.detail}</p>
									</Card.Content>
								</Card.Root>
							{/each}
						</div>
					{/if}
				</Card.Content>
			</Card.Root>
		</div>

		{#if selectedHost}
			<div class="grid gap-4 xl:grid-cols-2">
				{#each hostCharts as chart}
					<MetricChart series={chart.series} metricName={chart.title} loading={chart.loading} />
				{/each}
			</div>
		{/if}
	</div>
</PageContainer>
