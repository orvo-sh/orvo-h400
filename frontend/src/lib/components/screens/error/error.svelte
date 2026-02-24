<script lang="ts">
	let {
		message,
		requestId
	}: {
		message: string;
		requestId: string | null;
	} = $props();
</script>

<div>
	An error occured {message}
	{requestId}
</div>
