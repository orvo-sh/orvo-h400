<script lang="ts">
	import type { Timeseries } from '$lib/api/model';

	let {
		series,
		metricName,
		loading
	}: {
		series: Timeseries[];
		metricName: string;
		loading: boolean;
	} = $props();

	// Chart colors for multiple series
	const colors = [
		'#2563eb',
		'#10b981',
		'#f59e0b',
		'#ef4444',
		'#8b5cf6',
		'#ec4899',
		'#06b6d4',
		'#84cc16'
	];

	// Compute chart bounds from all series
	const chartData = $derived.by(() => {
		if (series.length === 0) return { allTimes: [] as Date[], yMin: 0, yMax: 1, seriesData: [] as { label: string; color: string; points: { time: Date; value: number }[] }[] };

		let allTimesSet = new Set<string>();
		let yMin = Infinity;
		let yMax = -Infinity;

		const seriesData = series.map((s, i) => {
			const points = (s.points ?? []).map((p) => {
				const time = new Date(p.time);
				allTimesSet.add(time.toISOString());
				if (p.value < yMin) yMin = p.value;
				if (p.value > yMax) yMax = p.value;
				return { time, value: p.value };
			});

			const labelParts = Object.entries(s.labels ?? {})
				.filter(([k]) => k !== '__name__')
				.map(([k, v]) => `${k}=${v}`);
			const label = labelParts.length > 0 ? labelParts.join(', ') : metricName;

			return { label, color: colors[i % colors.length], points };
		});

		if (yMin === Infinity) yMin = 0;
		if (yMax === -Infinity) yMax = 1;

		// Add 10% padding
		const range = yMax - yMin || 1;
		yMin = yMin - range * 0.05;
		yMax = yMax + range * 0.1;

		const allTimes = Array.from(allTimesSet)
			.map((t) => new Date(t))
			.sort((a, b) => a.getTime() - b.getTime());

		return { allTimes, yMin, yMax, seriesData };
	});

	function formatValue(v: number): string {
		if (Math.abs(v) >= 1e9) return (v / 1e9).toFixed(1) + 'B';
		if (Math.abs(v) >= 1e6) return (v / 1e6).toFixed(1) + 'M';
		if (Math.abs(v) >= 1e3) return (v / 1e3).toFixed(1) + 'K';
		if (Math.abs(v) < 0.01 && v !== 0) return v.toExponential(1);
		return v.toFixed(2);
	}

	function formatTime(d: Date): string {
		return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	}

	// SVG chart dimensions
	const width = 800;
	const height = 300;
	const padding = { top: 20, right: 20, bottom: 30, left: 60 };
	const chartW = width - padding.left - padding.right;
	const chartH = height - padding.top - padding.bottom;

	function xScale(d: Date): number {
		const { allTimes } = chartData;
		if (allTimes.length < 2) return padding.left;
		const min = allTimes[0].getTime();
		const max = allTimes[allTimes.length - 1].getTime();
		const range = max - min || 1;
		return padding.left + ((d.getTime() - min) / range) * chartW;
	}

	function yScale(v: number): number {
		const { yMin, yMax } = chartData;
		const range = yMax - yMin || 1;
		return padding.top + chartH - ((v - yMin) / range) * chartH;
	}

	function pathForSeries(points: { time: Date; value: number }[]): string {
		if (points.length === 0) return '';
		return points
			.map((p, i) => `${i === 0 ? 'M' : 'L'}${xScale(p.time).toFixed(1)},${yScale(p.value).toFixed(1)}`)
			.join(' ');
	}

	// Y axis ticks
	const yTicks = $derived.by(() => {
		const { yMin, yMax } = chartData;
		const count = 5;
		const step = (yMax - yMin) / count;
		return Array.from({ length: count + 1 }, (_, i) => yMin + step * i);
	});

	// X axis ticks (up to 8)
	const xTicks = $derived.by(() => {
		const { allTimes } = chartData;
		if (allTimes.length === 0) return [];
		const count = Math.min(8, allTimes.length);
		const step = Math.max(1, Math.floor(allTimes.length / count));
		const ticks: Date[] = [];
		for (let i = 0; i < allTimes.length; i += step) {
			ticks.push(allTimes[i]);
		}
		return ticks;
	});

	// Tooltip state
	let tooltipVisible = $state(false);
	let tooltipX = $state(0);
	let tooltipY = $state(0);
	let tooltipContent = $state<{ label: string; value: string; color: string }[]>([]);
	let tooltipTime = $state('');

	function handleMouseMove(e: MouseEvent) {
		const svg = (e.currentTarget as SVGSVGElement);
		const rect = svg.getBoundingClientRect();
		const svgX = ((e.clientX - rect.left) / rect.width) * width;

		const { allTimes, seriesData } = chartData;
		if (allTimes.length === 0) return;

		// Find closest time
		let closestIdx = 0;
		let closestDist = Infinity;
		for (let i = 0; i < allTimes.length; i++) {
			const dist = Math.abs(xScale(allTimes[i]) - svgX);
			if (dist < closestDist) {
				closestDist = dist;
				closestIdx = i;
			}
		}

		const time = allTimes[closestIdx];
		tooltipTime = formatTime(time);
		tooltipX = e.clientX - rect.left;
		tooltipY = e.clientY - rect.top;
		tooltipContent = seriesData.map((s) => {
			const point = s.points.find((p) => p.time.getTime() === time.getTime());
			return {
				label: s.label,
				value: point ? formatValue(point.value) : '-',
				color: s.color
			};
		});
		tooltipVisible = true;
	}

	function handleMouseLeave() {
		tooltipVisible = false;
	}
</script>

<div class="relative flex flex-1 flex-col rounded-lg border bg-card">
	<div class="flex items-center justify-between border-b px-4 py-2">
		<h3 class="text-sm font-medium">{metricName}</h3>
		{#if series.length > 1}
			<span class="text-xs text-muted-foreground">{series.length} series</span>
		{/if}
	</div>

	{#if loading}
		<div class="flex flex-1 items-center justify-center py-16">
			<p class="text-sm text-muted-foreground">Loading...</p>
		</div>
	{:else if series.length === 0}
		<div class="flex flex-1 items-center justify-center py-16">
			<p class="text-sm text-muted-foreground">No data for the selected time range</p>
		</div>
	{:else}
		<div class="flex-1 p-4">
			<!-- Legend -->
			{#if chartData.seriesData.length > 1}
				<div class="mb-2 flex flex-wrap gap-3">
					{#each chartData.seriesData as s}
						<div class="flex items-center gap-1.5 text-xs">
							<div class="size-2.5 rounded-full" style="background-color: {s.color}"></div>
							<span class="text-muted-foreground">{s.label}</span>
						</div>
					{/each}
				</div>
			{/if}

			<svg
				viewBox="0 0 {width} {height}"
				class="h-full w-full"
				role="img"
				onmousemove={handleMouseMove}
				onmouseleave={handleMouseLeave}
			>
				<!-- Y axis gridlines -->
				{#each yTicks as tick}
					<line
						x1={padding.left}
						x2={width - padding.right}
						y1={yScale(tick)}
						y2={yScale(tick)}
						stroke="currentColor"
						stroke-opacity="0.1"
					/>
					<text
						x={padding.left - 8}
						y={yScale(tick)}
						text-anchor="end"
						dominant-baseline="middle"
						class="fill-muted-foreground text-[10px]"
					>
						{formatValue(tick)}
					</text>
				{/each}

				<!-- X axis labels -->
				{#each xTicks as tick}
					<text
						x={xScale(tick)}
						y={height - 5}
						text-anchor="middle"
						class="fill-muted-foreground text-[10px]"
					>
						{formatTime(tick)}
					</text>
				{/each}

				<!-- Data lines -->
				{#each chartData.seriesData as s}
					<path
						d={pathForSeries(s.points)}
						fill="none"
						stroke={s.color}
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
					{#if s.points.length <= 1}
						{#each s.points as p}
							<circle
								cx={xScale(p.time)}
								cy={yScale(p.value)}
								r="3"
								fill={s.color}
							/>
						{/each}
					{/if}
				{/each}
			</svg>

			<!-- Tooltip -->
			{#if tooltipVisible}
				<div
					class="pointer-events-none absolute z-50 rounded-md border bg-popover px-3 py-2 text-xs shadow-md"
					style="left: {tooltipX + 12}px; top: {tooltipY - 10}px"
				>
					<div class="mb-1 font-medium">{tooltipTime}</div>
					{#each tooltipContent as item}
						<div class="flex items-center gap-1.5">
							<div class="size-2 rounded-full" style="background-color: {item.color}"></div>
							<span class="text-muted-foreground">{item.label}:</span>
							<span class="font-mono">{item.value}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>
