<script lang="ts">
	import type { TimeseriesPoint } from '$lib/api/model';

	let {
		title,
		points,
		secondaryPoints,
		color,
		secondaryColor,
		loading,
		thresholdValue = undefined,
		thresholdLabel = 'Threshold',
		interactive = false,
		onSelectPoint = undefined
	}: {
		title: string;
		points: TimeseriesPoint[];
		secondaryPoints?: TimeseriesPoint[];
		color: string;
		secondaryColor?: string;
		loading: boolean;
		thresholdValue?: number;
		thresholdLabel?: string;
		interactive?: boolean;
		onSelectPoint?: (point: TimeseriesPoint) => void;
	} = $props();

	const width = 400;
	const height = 180;
	const padding = { top: 12, right: 12, bottom: 24, left: 50 };
	const chartW = width - padding.left - padding.right;
	const chartH = height - padding.top - padding.bottom;

	const chartData = $derived.by(() => {
		const all = [...points, ...(secondaryPoints ?? [])];
		if (Number.isFinite(thresholdValue)) {
			all.push({
				time: points[points.length - 1]?.time ?? new Date().toISOString(),
				value: thresholdValue ?? 0
			});
		}

		if (all.length === 0) {
			return { times: [] as Date[], yMin: 0, yMax: 1 };
		}

		let yMin = Infinity;
		let yMax = -Infinity;
		for (const point of all) {
			if (point.value < yMin) yMin = point.value;
			if (point.value > yMax) yMax = point.value;
		}
		if (yMin === Infinity) yMin = 0;
		if (yMax === -Infinity) yMax = 1;
		const range = yMax - yMin || 1;
		yMin -= range * 0.05;
		yMax += range * 0.1;

		const times = points.map((point) => new Date(point.time)).sort((a, b) => a.getTime() - b.getTime());
		return { times, yMin, yMax };
	});

	function xScale(date: Date): number {
		const { times } = chartData;
		if (times.length < 2) return padding.left;
		const min = times[0].getTime();
		const max = times[times.length - 1].getTime();
		return padding.left + ((date.getTime() - min) / (max - min || 1)) * chartW;
	}

	function yScale(value: number): number {
		const { yMin, yMax } = chartData;
		return padding.top + chartH - ((value - yMin) / (yMax - yMin || 1)) * chartH;
	}

	function buildPath(values: TimeseriesPoint[]): string {
		if (values.length === 0) return '';
		return values
			.map(
				(point, index) =>
					`${index === 0 ? 'M' : 'L'}${xScale(new Date(point.time)).toFixed(1)},${yScale(point.value).toFixed(1)}`
			)
			.join(' ');
	}

	function formatValue(value: number): string {
		if (Math.abs(value) >= 1e6) return (value / 1e6).toFixed(1) + 'M';
		if (Math.abs(value) >= 1e3) return (value / 1e3).toFixed(1) + 'K';
		if (Math.abs(value) < 0.01 && value !== 0) return value.toExponential(1);
		return value.toFixed(2);
	}

	function formatTime(date: Date): string {
		return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	}

	function handleChartClick(event: MouseEvent) {
		if (!interactive || !onSelectPoint || points.length === 0) {
			return;
		}

		const target = event.currentTarget as HTMLButtonElement;
		const rect = target.getBoundingClientRect();
		const relativeX = Math.min(Math.max(event.clientX - rect.left, 0), rect.width);
		const svgX = (relativeX / rect.width) * width;

		let closestPoint: TimeseriesPoint | null = null;
		let closestDistance = Infinity;
		for (const point of points) {
			const distance = Math.abs(xScale(new Date(point.time)) - svgX);
			if (distance < closestDistance) {
				closestDistance = distance;
				closestPoint = point;
			}
		}

		if (closestPoint) {
			onSelectPoint(closestPoint);
		}
	}

	function handleChartKeydown(event: KeyboardEvent) {
		if (!interactive || !onSelectPoint || points.length === 0) {
			return;
		}
		if (event.key !== 'Enter' && event.key !== ' ') {
			return;
		}
		event.preventDefault();
		onSelectPoint(points[points.length - 1]);
	}

	const yTicks = $derived.by(() => {
		const { yMin, yMax } = chartData;
		const count = 4;
		const step = (yMax - yMin) / count;
		return Array.from({ length: count + 1 }, (_, index) => yMin + step * index);
	});

	const xTicks = $derived.by(() => {
		const { times } = chartData;
		if (times.length === 0) return [];
		const count = Math.min(5, times.length);
		const step = Math.max(1, Math.floor(times.length / count));
		const ticks: Date[] = [];
		for (let index = 0; index < times.length; index += step) {
			ticks.push(times[index]);
		}
		return ticks;
	});
</script>

<div class="rounded-lg border bg-card">
	<div class="flex items-center justify-between border-b px-3 py-2">
		<h4 class="text-xs font-medium text-muted-foreground">{title}</h4>
		{#if interactive}
			<span class="rounded-full bg-red-100 px-2 py-1 text-[10px] font-semibold uppercase tracking-wide text-red-700">
				Click chart to seed threshold
			</span>
		{/if}
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<p class="text-xs text-muted-foreground">Loading...</p>
		</div>
	{:else if points.length === 0}
		<div class="flex items-center justify-center py-12">
			<p class="text-xs text-muted-foreground">No data</p>
		</div>
	{:else}
		{#snippet chartSvg()}
			<svg viewBox="0 0 {width} {height}" class={`h-full w-full ${interactive ? 'cursor-crosshair' : ''}`} role="img" aria-label={title}>
				{#each yTicks as tick}
					<line
						x1={padding.left}
						x2={width - padding.right}
						y1={yScale(tick)}
						y2={yScale(tick)}
						stroke="currentColor"
						stroke-opacity="0.08"
					/>
					<text
						x={padding.left - 6}
						y={yScale(tick)}
						text-anchor="end"
						dominant-baseline="middle"
						class="fill-muted-foreground text-[9px]"
					>
						{formatValue(tick)}
					</text>
				{/each}

				{#each xTicks as tick}
					<text
						x={xScale(tick)}
						y={height - 4}
						text-anchor="middle"
						class="fill-muted-foreground text-[9px]"
					>
						{formatTime(tick)}
					</text>
				{/each}

				{#if Number.isFinite(thresholdValue)}
					<line
						x1={padding.left}
						x2={width - padding.right}
						y1={yScale(thresholdValue ?? 0)}
						y2={yScale(thresholdValue ?? 0)}
						stroke="#ef4444"
						stroke-width="1.5"
						stroke-dasharray="6 4"
					/>
					<text
						x={width - padding.right}
						y={yScale(thresholdValue ?? 0) - 4}
						text-anchor="end"
						class="fill-red-500 text-[9px] font-medium"
					>
						{thresholdLabel}: {formatValue(thresholdValue ?? 0)}
					</text>
				{/if}

				<path d={buildPath(points)} fill="none" stroke={color} stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
				{#if interactive || points.length <= 1}
					{#each points as point}
						<circle
							cx={xScale(new Date(point.time))}
							cy={yScale(point.value)}
							r={interactive ? 3.5 : 3}
							fill={color}
							fill-opacity={interactive ? 0.9 : 1}
						/>
					{/each}
				{/if}

				{#if secondaryPoints && secondaryPoints.length > 0}
					<path
						d={buildPath(secondaryPoints)}
						fill="none"
						stroke={secondaryColor ?? '#999'}
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
						stroke-dasharray="4 2"
					/>
					{#if secondaryPoints.length <= 1}
						{#each secondaryPoints as point}
							<circle cx={xScale(new Date(point.time))} cy={yScale(point.value)} r="3" fill={secondaryColor ?? '#999'} />
						{/each}
					{/if}
				{/if}
			</svg>
		{/snippet}

		<div class="p-2">
			{#if interactive}
				<button
					type="button"
					class="block w-full p-0 text-left"
					aria-label={`${title}. Click a point to seed the threshold, or press Enter to use the latest point.`}
					onclick={handleChartClick}
					onkeydown={handleChartKeydown}
				>
					{@render chartSvg()}
				</button>
			{:else}
				{@render chartSvg()}
			{/if}
		</div>
	{/if}
</div>
