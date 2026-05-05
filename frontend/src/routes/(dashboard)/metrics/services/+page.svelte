<script lang="ts">
	import PageContainer from '../../_components/page-container/page-container.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import * as Select from '$lib/components/ui/select/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import ActivityIcon from '@lucide/svelte/icons/activity';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import RedChart from './_components/red-chart.svelte';
	import {
		createGetRedMetrics,
		recalculateDerivedMetrics
	} from '$lib/api/endpoints/metrics/metrics';
	import { createGetTraceServices } from '$lib/api/endpoints/traces/traces';
	import {
		deleteAutoResolveThreshold,
		listAutoResolveThresholds,
		listServiceRemediationMappings,
		RemediationAPIError,
		type AutoResolveThreshold,
		type ServiceRemediationMapping,
		upsertAutoResolveThreshold
	} from '$lib/remediation/api';
	import { sessionStore } from '$lib/stores/session';
	import type { TimeseriesPoint } from '$lib/api/model';

	type ThresholdWindow = '10s' | '30s' | '1m' | '5m' | '15m';
	type ThresholdCooldown = '30s' | '1m' | '5m' | '15m';

	let selectedService = $state('');
	let selectedTimeRange = $state('1h');
	let recalculating = $state(false);
	let recalcStatus = $state('');
	let thresholdsLoading = $state(false);
	let thresholdsSaving = $state(false);
	let thresholdsDeleting = $state(false);
	let thresholdsError = $state<string | null>(null);
	let thresholdHint = $state('Click the Error Rate chart to pick a threshold.');
	let remediationMappings = $state<ServiceRemediationMapping[]>([]);
	let autoResolveThresholds = $state<AutoResolveThreshold[]>([]);
	let thresholdValueInput = $state('');
	let thresholdWindow = $state<ThresholdWindow>('1m');
	let thresholdCooldown = $state<ThresholdCooldown>('30s');
	let thresholdQuorum = $state('5');
	let thresholdEnabled = $state(true);

	const thresholdWindowSeconds: Record<ThresholdWindow, number> = {
		'10s': 10,
		'30s': 30,
		'1m': 60,
		'5m': 300,
		'15m': 900
	};

	const orgId = $derived($sessionStore?.active_organization?.id ?? '');

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
		const ms = timeRangeMs[range] ?? 3_600_000;
		return {
			start: new Date(now.getTime() - ms).toISOString(),
			end: now.toISOString()
		};
	}

	const initialRange = computeTimeRange('1h');
	let queryStart = $state(initialRange.start);
	let queryEnd = $state(initialRange.end);

	function refreshQueries() {
		const range = computeTimeRange(selectedTimeRange);
		queryStart = range.start;
		queryEnd = range.end;
	}

	function rangeToLookback(range: string): string {
		switch (range) {
			case '15m':
				return '15m';
			case '30m':
				return '30m';
			case '1h':
				return '1h';
			case '3h':
				return '3h';
			case '6h':
				return '6h';
			case '12h':
				return '12h';
			case '24h':
				return '24h';
			case '7d':
				return '168h';
			default:
				return '1h';
		}
	}

	$effect(() => {
		selectedTimeRange;
		refreshQueries();
	});

	const servicesQuery = createGetTraceServices(() => orgId);

	const services = $derived.by((): string[] => {
		const response = servicesQuery.data;
		if (response && response.status === 200) {
			return response.data.services ?? [];
		}
		return [];
	});

	$effect(() => {
		if (!selectedService && services.length > 0) {
			selectedService = services[0];
		}
	});

	const redQuery = createGetRedMetrics(
		() => orgId,
		() => ({
			service: selectedService,
			start: queryStart,
			end: queryEnd,
			step: defaultSteps[selectedTimeRange] || '1m'
		}),
		() => ({
			query: {
				enabled: !!selectedService && !!orgId
			}
		})
	);

	const requestRate = $derived.by((): TimeseriesPoint[] => {
		const response = redQuery.data;
		if (response && response.status === 200) return response.data.request_rate ?? [];
		return [];
	});

	const errorRate = $derived.by((): TimeseriesPoint[] => {
		const response = redQuery.data;
		if (response && response.status === 200) return response.data.error_rate ?? [];
		return [];
	});

	const p50 = $derived.by((): TimeseriesPoint[] => {
		const response = redQuery.data;
		if (response && response.status === 200) return response.data.p50_latency ?? [];
		return [];
	});

	const p90 = $derived.by((): TimeseriesPoint[] => {
		const response = redQuery.data;
		if (response && response.status === 200) return response.data.p90_latency ?? [];
		return [];
	});

	const p95 = $derived.by((): TimeseriesPoint[] => {
		const response = redQuery.data;
		if (response && response.status === 200) return response.data.p95_latency ?? [];
		return [];
	});

	const p99 = $derived.by((): TimeseriesPoint[] => {
		const response = redQuery.data;
		if (response && response.status === 200) return response.data.p99_latency ?? [];
		return [];
	});

	const latestErrorPoint = $derived.by((): TimeseriesPoint | null => {
		if (errorRate.length === 0) return null;
		return errorRate[errorRate.length - 1];
	});

	function normalizeServiceKey(value: string): string {
		return value
			.trim()
			.toLowerCase()
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-+|-+$/g, '');
	}

	function findByServiceName<T extends { service_name: string }>(items: T[], serviceName: string): T | null {
		const exact = items.find((item) => item.service_name === serviceName);
		if (exact) {
			return exact;
		}

		const normalizedServiceName = normalizeServiceKey(serviceName);
		if (!normalizedServiceName) {
			return null;
		}

		return items.find((item) => normalizeServiceKey(item.service_name) === normalizedServiceName) ?? null;
	}

	const selectedServiceThreshold = $derived.by((): AutoResolveThreshold | null => {
		if (!selectedService) return null;
		return findByServiceName(autoResolveThresholds, selectedService);
	});

	const selectedServiceMapping = $derived.by((): ServiceRemediationMapping | null => {
		if (!selectedService) return null;
		return findByServiceName(remediationMappings, selectedService);
	});

	const thresholdValue = $derived.by(() => {
		const parsed = Number.parseFloat(thresholdValueInput);
		return Number.isFinite(parsed) ? parsed : undefined;
	});

	const thresholdStatus = $derived.by((): { label: string; variant: 'outline' | 'secondary' } => {
		if (!selectedServiceThreshold) {
			return { label: 'Not saved', variant: 'outline' };
		}

		if (!selectedServiceThreshold.enabled) {
			return { label: 'Paused', variant: 'outline' };
		}

		if (!selectedServiceMapping) {
			return { label: 'Needs mapping', variant: 'outline' };
		}

		return { label: 'Active', variant: 'secondary' };
	});

	$effect(() => {
		const currentOrgId = orgId;
		if (!currentOrgId) {
			remediationMappings = [];
			autoResolveThresholds = [];
			return;
		}
		void loadAutomationConfig(currentOrgId);
	});

	$effect(() => {
		selectedService;
		selectedServiceThreshold;
		syncThresholdForm();
	});

	function lastValue(points: TimeseriesPoint[]): string {
		if (points.length === 0) return '-';
		const value = points[points.length - 1].value;
		if (Math.abs(value) >= 1000) return (value / 1000).toFixed(1) + 'K';
		return value.toFixed(2);
	}

	function formatRate(value: number | undefined): string {
		if (value === undefined || !Number.isFinite(value)) return '0.00';
		return value.toFixed(2);
	}

	function formatErrorCount(value: number | undefined): string {
		if (value === undefined || !Number.isFinite(value)) return '0';
		return Math.max(1, Math.ceil(value)).toString();
	}

	function thresholdWindowToSeconds(window: ThresholdWindow): number {
		return thresholdWindowSeconds[window];
	}

	function ratePointToErrorCount(point: TimeseriesPoint, window: ThresholdWindow): number {
		return Math.max(1, Math.ceil(point.value * thresholdWindowToSeconds(window)));
	}

	function syncThresholdForm() {
		const threshold = selectedServiceThreshold;
		if (!threshold) {
			thresholdValueInput = '';
			thresholdWindow = '1m';
			thresholdCooldown = '30s';
			thresholdQuorum = '5';
			thresholdEnabled = true;
			thresholdHint = 'Click the Error Rate chart to pick a threshold.';
			return;
		}

		thresholdValueInput = formatErrorCount(threshold.threshold_value);
		thresholdWindow = normalizeThresholdWindow(threshold.lookback_window);
		thresholdCooldown = normalizeThresholdCooldown(threshold.cooldown);
		thresholdQuorum = String(threshold.quorum);
		thresholdEnabled = threshold.enabled;
		if (!threshold.enabled) {
			thresholdHint = 'Saved threshold is paused.';
			return;
		}

		thresholdHint = selectedServiceMapping
			? 'Saved threshold is active.'
			: 'Saved threshold needs a repository mapping before Auto Resolve can run.';
	}

	function normalizeThresholdWindow(value: string): ThresholdWindow {
		if (value === '10s' || value === '30s' || value === '1m' || value === '5m' || value === '15m') {
			return value;
		}
		return '1m';
	}

	function normalizeThresholdCooldown(value: string): ThresholdCooldown {
		if (value === '30s' || value === '1m' || value === '5m' || value === '15m') {
			return value;
		}
		return '30s';
	}

	async function loadAutomationConfig(currentOrgId: string) {
		thresholdsLoading = true;
		thresholdsError = null;
		const [mappingsResult, thresholdsResult] = await Promise.allSettled([
			listServiceRemediationMappings(currentOrgId),
			listAutoResolveThresholds(currentOrgId)
		]);
		try {
			if (mappingsResult.status === 'fulfilled') {
				remediationMappings = mappingsResult.value;
			} else {
				remediationMappings = [];
				thresholdsError =
					mappingsResult.reason instanceof Error
						? mappingsResult.reason.message
						: 'failed to load repository mappings';
			}

			if (thresholdsResult.status === 'fulfilled') {
				autoResolveThresholds = thresholdsResult.value;
			} else {
				autoResolveThresholds = [];
				thresholdsError =
					thresholdsResult.reason instanceof Error
						? thresholdsResult.reason.message
						: 'failed to load threshold settings';
			}
		} finally {
			thresholdsLoading = false;
		}
	}

	function handleThresholdPointSelect(point: TimeseriesPoint) {
		const errorCount = ratePointToErrorCount(point, thresholdWindow);
		thresholdValueInput = String(errorCount);
		thresholdHint =
			`Selected about ${errorCount} errors in ${thresholdWindow} from ` +
			`${new Date(point.time).toLocaleTimeString()} (${formatRate(point.value)} err/s).`;
		thresholdsError = null;
	}

	function seedThresholdFromLatest() {
		if (!latestErrorPoint) {
			return;
		}

		handleThresholdPointSelect(latestErrorPoint);
	}

	async function refreshAll() {
		refreshQueries();
		await Promise.all([
			redQuery.refetch(),
			orgId ? loadAutomationConfig(orgId) : Promise.resolve()
		]);
	}

	async function recalculateDerivedNow() {
		if (!orgId || recalculating) {
			return;
		}

		recalculating = true;
		recalcStatus = '';
		try {
			const response = await recalculateDerivedMetrics(orgId, {
				service: selectedService || undefined,
				lookback: rangeToLookback(selectedTimeRange)
			});
			await refreshAll();

			if (response.status === 200) {
				const asOf = response.data.as_of ? new Date(response.data.as_of).toLocaleTimeString() : 'now';
				recalcStatus = `Derived metrics recalculated (as of ${asOf})`;
			} else {
				recalcStatus = 'Derived metrics recalculation failed';
			}
		} catch {
			recalcStatus = 'Derived metrics recalculation failed';
		} finally {
			recalculating = false;
		}
	}

	async function saveThreshold() {
		if (!orgId || !selectedService) {
			return;
		}

		const target = Number.parseFloat(thresholdValueInput);
		const quorum = Number.parseInt(thresholdQuorum, 10);
		if (!Number.isFinite(target) || target <= 0) {
			thresholdsError = 'Pick a threshold from the chart or enter a value above 0.';
			return;
		}
		if (!Number.isFinite(quorum) || quorum <= 0) {
			thresholdsError = 'Failed requests must be at least 1.';
			return;
		}
		if (thresholdEnabled && !selectedServiceMapping) {
			thresholdsError = 'Map this service to a repository before arming Auto Resolve.';
			return;
		}

		thresholdsSaving = true;
		thresholdsError = null;
		try {
			await upsertAutoResolveThreshold(orgId, selectedService, {
				threshold_value: target,
				lookback_window: thresholdWindow,
				cooldown: thresholdCooldown,
				quorum,
				enabled: thresholdEnabled
			});
			await loadAutomationConfig(orgId);
			thresholdHint = thresholdEnabled
				? 'Threshold saved. Auto Resolve is now watching this service.'
				: 'Threshold saved, but it is paused.';
		} catch (error) {
			if (error instanceof RemediationAPIError) {
				if (error.code === 'auto_resolve_service_mapping_missing') {
					thresholdsError = 'Map this service to a repository before arming Auto Resolve.';
				} else if (error.code === 'auto_resolve_opencode_not_configured') {
					thresholdsError = 'Auto Resolve is not configured on the server yet.';
				} else {
					thresholdsError = error.message;
				}
			} else {
				thresholdsError = error instanceof Error ? error.message : 'failed to save threshold';
			}
		} finally {
			thresholdsSaving = false;
		}
	}

	async function removeThreshold() {
		if (!orgId || !selectedService || !selectedServiceThreshold) {
			return;
		}

		thresholdsDeleting = true;
		thresholdsError = null;
		try {
			await deleteAutoResolveThreshold(orgId, selectedService);
			await loadAutomationConfig(orgId);
			thresholdValueInput = '';
			thresholdHint = 'Threshold removed.';
		} catch (error) {
			thresholdsError = error instanceof Error ? error.message : 'failed to remove threshold';
		} finally {
			thresholdsDeleting = false;
		}
	}

</script>

<PageContainer breadcrumbs={[{ title: 'Metrics', href: '/metrics' }, { title: 'Services' }]}>
	<div class="flex h-full flex-col gap-4">
		<div class="flex items-start justify-between">
			<div class="flex items-center gap-3">
				<div class="rounded-lg bg-primary/10 p-2">
					<ActivityIcon class="size-6 text-primary" />
				</div>
				<div>
					<h1 class="text-xl font-semibold">Service Overview</h1>
					<p class="text-sm text-muted-foreground">Rate, Errors, and Duration (RED) metrics for your services</p>
				</div>
			</div>
		</div>

		<Separator />

		<div class="flex flex-wrap items-center gap-3">
			<Select.Root
				type="single"
				value={selectedService}
				onValueChange={(value) => {
					if (value) selectedService = value;
				}}
			>
				<Select.Trigger class="w-64">
					{selectedService || 'Select a service'}
				</Select.Trigger>
				<Select.Content>
					{#each services as service}
						<Select.Item value={service}>{service}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>

			<Select.Root
				type="single"
				value={selectedTimeRange}
				onValueChange={(value) => {
					if (value) selectedTimeRange = value;
				}}
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

			<Button variant="outline" onclick={recalculateDerivedNow} disabled={recalculating || !orgId}>
				<RefreshCwIcon class="mr-2 size-4 {recalculating ? 'animate-spin' : ''}" />
				Recalculate Derived
			</Button>
			<Button onclick={refreshAll}>
				<RefreshCwIcon class="mr-2 size-4" />
				Refresh
			</Button>
		</div>

		{#if recalcStatus}
			<p class="text-xs text-muted-foreground">{recalcStatus}</p>
		{/if}

		{#if !selectedService}
			<div class="flex flex-1 items-center justify-center rounded-lg border border-dashed">
				<p class="text-sm text-muted-foreground">Select a service to view RED metrics</p>
			</div>
		{:else}
			<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
				<Card.Root>
					<Card.Header class="pb-2">
						<Card.Title class="text-sm font-medium text-muted-foreground">Request Rate</Card.Title>
					</Card.Header>
					<Card.Content>
						<p class="text-2xl font-bold">{lastValue(requestRate)}<span class="text-sm font-normal text-muted-foreground"> req/s</span></p>
					</Card.Content>
				</Card.Root>
				<Card.Root>
					<Card.Header class="pb-2">
						<Card.Title class="text-sm font-medium text-muted-foreground">Error Rate</Card.Title>
					</Card.Header>
					<Card.Content>
						<p class="text-2xl font-bold">{lastValue(errorRate)}<span class="text-sm font-normal text-muted-foreground"> err/s</span></p>
					</Card.Content>
				</Card.Root>
				<Card.Root>
					<Card.Header class="pb-2">
						<Card.Title class="text-sm font-medium text-muted-foreground">P50 Latency</Card.Title>
					</Card.Header>
					<Card.Content>
						<p class="text-2xl font-bold">{lastValue(p50)}<span class="text-sm font-normal text-muted-foreground"> ms</span></p>
					</Card.Content>
				</Card.Root>
				<Card.Root>
					<Card.Header class="pb-2">
						<Card.Title class="text-sm font-medium text-muted-foreground">P99 Latency</Card.Title>
					</Card.Header>
					<Card.Content>
						<p class="text-2xl font-bold">{lastValue(p99)}<span class="text-sm font-normal text-muted-foreground"> ms</span></p>
					</Card.Content>
				</Card.Root>
			</div>

			<div class="grid gap-4 xl:grid-cols-[minmax(0,1.35fr)_22rem]">
				<RedChart
					title="Error Rate (err/s)"
					points={errorRate}
					color="#ef4444"
					loading={redQuery.isPending}
					thresholdValue={thresholdValue}
					thresholdLabel="Auto-resolve threshold"
					interactive={true}
					onSelectPoint={handleThresholdPointSelect}
				/>

				<Card.Root class="border-primary/20">
					<Card.Header class="gap-2">
						<div class="flex items-center justify-between gap-2">
							<Card.Title class="text-base">Auto-resolve threshold</Card.Title>
							<Badge variant={thresholdStatus.variant}>{thresholdStatus.label}</Badge>
						</div>
						<Card.Description>
							Save the number of errors you want to tolerate inside the selected window. Auto
							Resolve can start after the service reaches that error count, enough requests fail,
							and the service is mapped to a repository.
						</Card.Description>
					</Card.Header>
					<Card.Content class="space-y-4">
						<div class="rounded-md border bg-muted/30 p-3 text-sm">
							<p class="font-medium text-foreground">How this works</p>
							<p class="mt-1 text-muted-foreground">
								Threshold:
								<span class="font-medium text-foreground">
									{formatErrorCount(thresholdValue)} errors per {thresholdWindow}
								</span>
							</p>
							<p class="text-muted-foreground">
								Window: <span class="font-medium text-foreground">{thresholdWindow}</span>
							</p>
							<p class="text-muted-foreground">
								Failed requests: <span class="font-medium text-foreground">{thresholdQuorum}</span>
							</p>
						</div>

						<div class="space-y-2">
							<Label for="threshold-value">Error count threshold</Label>
							<Input
								id="threshold-value"
								type="number"
								min="1"
								step="1"
								placeholder="Click the chart to estimate this"
								bind:value={thresholdValueInput}
							/>
						</div>

						<div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
							<div class="space-y-2">
								<Label>Breach window</Label>
								<Select.Root
									type="single"
									value={thresholdWindow}
									onValueChange={(value) => {
										if (value) thresholdWindow = value as ThresholdWindow;
									}}
								>
									<Select.Trigger>{thresholdWindow}</Select.Trigger>
									<Select.Content>
										{#each ['10s', '30s', '1m', '5m', '15m'] as window}
											<Select.Item value={window}>{window}</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
							</div>

							<div class="space-y-2">
								<Label for="threshold-quorum">Failed requests</Label>
								<Input id="threshold-quorum" type="number" min="1" step="1" bind:value={thresholdQuorum} />
							</div>
						</div>

						<label class="flex items-center gap-2 text-sm text-muted-foreground">
							<input type="checkbox" bind:checked={thresholdEnabled} />
							Arm this threshold immediately
						</label>

						<div class="flex flex-wrap gap-2">
							<Button variant="outline" onclick={seedThresholdFromLatest} disabled={!latestErrorPoint}>
								Use latest sample
							</Button>
							<Button onclick={saveThreshold} disabled={thresholdsSaving || thresholdsDeleting}>
								{thresholdsSaving
									? 'Saving...'
									: selectedServiceThreshold
										? 'Update threshold'
										: 'Save threshold'}
							</Button>
							<Button
								variant="ghost"
								onclick={removeThreshold}
								disabled={!selectedServiceThreshold || thresholdsDeleting || thresholdsSaving}
							>
								{thresholdsDeleting ? 'Removing...' : 'Remove'}
							</Button>
						</div>

						<p class="text-xs text-muted-foreground">{thresholdHint}</p>

						{#if selectedServiceMapping}
							<p class="text-xs text-muted-foreground">
								Auto Resolve will open draft PRs in
								<span class="font-medium text-foreground">{selectedServiceMapping.repository_full_name}</span>.
							</p>
						{:else if !thresholdsLoading}
							<p class="text-xs text-amber-700">
								No repository is mapped to this service yet. Auto Resolve cannot arm this threshold until a mapping exists in
								<a class="underline" href="/settings#remediation-mappings">Settings</a>.
							</p>
						{/if}

						{#if thresholdsLoading}
							<p class="text-xs text-muted-foreground">Loading threshold settings...</p>
						{/if}

						{#if thresholdsError}
							<p class="text-xs text-destructive">{thresholdsError}</p>
						{/if}
					</Card.Content>
				</Card.Root>
			</div>

			<div class="grid gap-4 lg:grid-cols-3">
				<RedChart title="Request Rate (req/s)" points={requestRate} color="#2563eb" loading={redQuery.isPending} />
				<RedChart
					title="Latency (P50 / P90)"
					points={p50}
					secondaryPoints={p90}
					color="#10b981"
					secondaryColor="#f59e0b"
					loading={redQuery.isPending}
				/>
				<RedChart
					title="Latency (P95 / P99)"
					points={p95}
					secondaryPoints={p99}
					color="#8b5cf6"
					secondaryColor="#ef4444"
					loading={redQuery.isPending}
				/>
			</div>
		{/if}
	</div>
</PageContainer>
