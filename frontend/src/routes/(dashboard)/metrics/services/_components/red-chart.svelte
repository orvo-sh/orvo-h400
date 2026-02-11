<script lang="ts">
	import type { TimeseriesPoint } from '$lib/api/model';

	let {
		title,
		points,
		secondaryPoints,
		color,
		secondaryColor,
		loading
	}: {
		title: string;
		points: TimeseriesPoint[];
		secondaryPoints?: TimeseriesPoint[];
		color: string;
		secondaryColor?: string;
		loading: boolean;
	} = $props();

	const width = 400;
	const height = 180;
	const padding = { top: 12, right: 12, bottom: 24, left: 50 };
	const chartW = width - padding.left - padding.right;
	const chartH = height - padding.top - padding.bottom;

	const chartData = $derived.by(() => {
		const all = [...points, ...(secondaryPoints ?? [])];
		if (all.length === 0) return { times: [] as Date[], yMin: 0, yMax: 1 };

		let yMin = Infinity;
		let yMax = -Infinity;
		for (const p of all) {
			if (p.value < yMin) yMin = p.value;
			if (p.value > yMax) yMax = p.value;
		}
		if (yMin === Infinity) yMin = 0;
		if (yMax === -Infinity) yMax = 1;
		const range = yMax - yMin || 1;
		yMin -= range * 0.05;
		yMax += range * 0.1;

		const times = points.map((p) => new Date(p.time)).sort((a, b) => a.getTime() - b.getTime());
		return { times, yMin, yMax };
	});

	function xScale(d: Date): number {
		const { times } = chartData;
		if (times.length < 2) return padding.left;
		const min = times[0].getTime();
		const max = times[times.length - 1].getTime();
		return padding.left + ((d.getTime() - min) / (max - min || 1)) * chartW;
	}

	function yScale(v: number): number {
		const { yMin, yMax } = chartData;
		return padding.top + chartH - ((v - yMin) / (yMax - yMin || 1)) * chartH;
	}

	function buildPath(pts: TimeseriesPoint[]): string {
		if (pts.length === 0) return '';
		return pts
			.map((p, i) => `${i === 0 ? 'M' : 'L'}${xScale(new Date(p.time)).toFixed(1)},${yScale(p.value).toFixed(1)}`)
			.join(' ');
	}

	function formatValue(v: number): string {
		if (Math.abs(v) >= 1e6) return (v / 1e6).toFixed(1) + 'M';
		if (Math.abs(v) >= 1e3) return (v / 1e3).toFixed(1) + 'K';
		if (Math.abs(v) < 0.01 && v !== 0) return v.toExponential(1);
		return v.toFixed(1);
	}

	function formatTime(d: Date): string {
		return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	}

	const yTicks = $derived.by(() => {
		const { yMin, yMax } = chartData;
		const count = 4;
		const step = (yMax - yMin) / count;
		return Array.from({ length: count + 1 }, (_, i) => yMin + step * i);
	});

	const xTicks = $derived.by(() => {
		const { times } = chartData;
		if (times.length === 0) return [];
		const count = Math.min(5, times.length);
		const step = Math.max(1, Math.floor(times.length / count));
		const ticks: Date[] = [];
		for (let i = 0; i < times.length; i += step) {
			ticks.push(times[i]);
		}
		return ticks;
	});
</script>

<div class="rounded-lg border bg-card">
	<div class="border-b px-3 py-2">
		<h4 class="text-xs font-medium text-muted-foreground">{title}</h4>
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
		<div class="p-2">
			<svg viewBox="0 0 {width} {height}" class="h-full w-full">
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

				<path d={buildPath(points)} fill="none" stroke={color} stroke-width="1.5" stroke-linejoin="round" />
				{#if secondaryPoints && secondaryPoints.length > 0}
					<path d={buildPath(secondaryPoints)} fill="none" stroke={secondaryColor ?? '#999'} stroke-width="1.5" stroke-linejoin="round" stroke-dasharray="4 2" />
				{/if}
			</svg>
		</div>
	{/if}
</div>
