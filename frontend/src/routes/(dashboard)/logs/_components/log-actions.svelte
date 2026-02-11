<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import LinkIcon from '@lucide/svelte/icons/link';
	import ClockIcon from '@lucide/svelte/icons/clock';
	import type { LogEntry } from './mock-data';

	let {
		log
	}: {
		log: LogEntry;
	} = $props();

	function copyAsJson() {
		navigator.clipboard.writeText(JSON.stringify(log, null, 2));
	}

	function copyLink() {
		const url = `${window.location.origin}/logs?id=${log.id}`;
		navigator.clipboard.writeText(url);
	}

	function openTimeDetective() {
		// This would open a time detective view in a real app
		console.log('Opening time detective for', log.timestamp);
	}
</script>

<div class="flex items-center gap-2 border-t bg-muted/30 px-4 py-3">
	<Button variant="outline" size="sm" onclick={copyAsJson}>
		<CopyIcon class="mr-1 size-4" />
		Copy as JSON
	</Button>
	<Button variant="outline" size="sm" onclick={copyLink}>
		<LinkIcon class="mr-1 size-4" />
		Copy link to log line
	</Button>
	<Button variant="outline" size="sm" onclick={openTimeDetective}>
		<ClockIcon class="mr-1 size-4" />
		Open time detective
	</Button>
</div>
