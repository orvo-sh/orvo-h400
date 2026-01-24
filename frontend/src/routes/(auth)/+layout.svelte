<script>
	import { goto } from '$app/navigation';
	import { ErrorScreen } from '$lib/components/screens/error';
	import { LoadingScreen } from '$lib/components/screens/loading';
	import { userState } from '$lib/state/user.svelte';

	let { children } = $props();
</script>

{#if userState._loading}
	<LoadingScreen message="Loading user data, please wait..." />
{:else if userState._error}
	<ErrorScreen message={userState._error.message} requestId={userState._error.requestId} />
{:else if userState.data}
	{goto('/')}
{:else}
	{@render children()}
{/if}
