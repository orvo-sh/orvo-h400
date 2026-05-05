<script lang="ts">
	import PageContainer from '../_components/page-container/page-container.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { createQueryTimeseries } from '$lib/api/endpoints/metrics/metrics';
	import { sessionStore } from '$lib/stores/session';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import ServerIcon from '@lucide/svelte/icons/server';
	import CpuIcon from '@lucide/svelte/icons/cpu';
	import MemoryStickIcon from '@lucide/svelte/icons/memory-stick';
	import Clock3Icon from '@lucide/svelte/icons/clock-3';
	import type { Timeseries, TimeseriesPoint } from '$lib/api/model';

	type HostSnapshot = {
		name: string;
		cpuUtilization: number | null;
		memoryUtilization: number | null;
		uptimeSeconds: number | null;
		lastSeen: string | null;
	};

	const hostObserverServiceName = 'orvo-host-observer';
	const lookbackMs = 10 * 60 * 1000;
	const orgId = $derived($sessionStore?.active_organization?.id ?? '');

	function buildWindow() {
		const now = new Date();
		return {
			start: new Date(now.getTime() - lookbackMs).toISOString(),
			end: now.toISOString()
		};
	}

	let queryWindow = $state(buildWindow());

	function refreshWindow() {
		queryWindow = buildWindow();
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

	const onlineHosts = $derived(
		hosts.filter((host) => {
			if (!host.lastSeen) return false;
			return Date.now() - new Date(host.lastSeen).getTime() < 3 * 60 * 1000;
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

	function hostStatusVariant(host: HostSnapshot): 'secondary' | 'outline' {
		if (!host.lastSeen) return 'outline';
		return Date.now() - new Date(host.lastSeen).getTime() < 3 * 60 * 1000 ? 'secondary' : 'outline';
	}

	function hostStatusLabel(host: HostSnapshot): string {
		if (!host.lastSeen) return 'Pending';
		return Date.now() - new Date(host.lastSeen).getTime() < 3 * 60 * 1000 ? 'Online' : 'Stale';
	}

	async function refreshAll() {
		refreshWindow();
		await Promise.all([cpuQuery.refetch(), memoryQuery.refetch(), uptimeQuery.refetch()]);
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
						Live infrastructure telemetry from the collector running on the deployment box.
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

		<Card.Root class="overflow-hidden">
			<Card.Header class="border-b bg-muted/20">
				<div class="flex items-center justify-between gap-4">
					<div>
						<Card.Title class="text-base">Host Inventory</Card.Title>
						<Card.Description>
							Each row combines CPU, memory, and uptime from the `orvo-host-observer` metric stream.
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
						<table class="w-full min-w-[720px] text-sm">
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
									<tr class="border-t">
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
	</div>
</PageContainer>
