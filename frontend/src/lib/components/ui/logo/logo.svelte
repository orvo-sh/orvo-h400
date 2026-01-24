<script lang="ts">
	import { cn } from '$lib/utils';

	let { class: className, ...restProps }: { class?: string } = $props();
</script>

<div class={cn('bg-primary size-8 rounded-md', className)} {...restProps}></div>
