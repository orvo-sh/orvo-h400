<script lang="ts">
	let {
		message,
		requestId
	}: {
		message: string;
		requestId: Nullable<string>;
	} = $props();
</script>

<div>
	An error occured {message}
	{requestId}
</div>
