<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	onMount(() => {
		goto('/sign-in');
	});
</script>

<svelte:head>
	<meta http-equiv="refresh" content="0;url=/sign-in" />
</svelte:head>

<div class="flex h-screen items-center justify-center">
	<a class="text-sm text-muted-foreground underline-offset-4 hover:underline" href="/sign-in">
		Continue to sign in
	</a>
</div>

