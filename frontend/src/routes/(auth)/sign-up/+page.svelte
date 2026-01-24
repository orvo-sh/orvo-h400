<script lang="ts">
	import * as Form from '$lib/components/ui/form';
	import { z } from 'zod';

	import { IconBrandGoogle } from '@tabler/icons-svelte';

	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Logo } from '$lib/components/ui/logo';
	import { superForm } from 'sveltekit-superforms';
	import { zodClient } from 'sveltekit-superforms/adapters';

	const form = superForm(
		{
			email: '',
			name: '',
			password: ''
		},
		{
			SPA: true,
			validators: zodClient(
				z.object({
					name: z.string().min(2).max(32),
					email: z.string().email(),
					password: z.string().min(8)
				})
			),
			onSubmit: async (formData) => {}
		}
	);

	let formData = form.form;
</script>

<div
	class="sm:flex-center border-primary relative h-screen min-h-screen flex-row gap-6 overflow-hidden p-0 pt-[10%] sm:pt-0 md:p-6 lg:gap-16"
>
	<div class="flex-center z-10 flex flex-col p-3 sm:p-4">
		<div class="text-foreground mb-4 flex items-center gap-2 text-xl font-bold tracking-tight">
			<Logo class="h-10 w-10" />
			<span class="mb-0.5 lg:block"> Panelist </span>
		</div>

		<div
			class="bg-elevated flex w-full flex-col gap-6   px-4 py-4 sm:w-[28rem] sm:max-w-[28rem]"
		>
			<div class="flex flex-col items-center space-y-0 text-center">
				<h1 class="text-foreground text-xl font-semibold tracking-tight">
					Get started with Panelist
				</h1>
				<p class="text-muted-foreground text-sm">continue with your email address or Google.</p>
			</div>

			<form method="POST" use:form.enhance>
				{#each ['name', 'email', 'password'] as const as f}
					<Form.Field name={f} {form}>
						<Form.Control>
							{#snippet children({ props })}
								<Form.Label>
									{f.charAt(0).toUpperCase() + f.slice(1)}
								</Form.Label>
								<Input {...props} bind:value={$formData[f]} type={f == 'name' ? 'text' : f} />
							{/snippet}
						</Form.Control>
						<Form.Description />
						<Form.FieldErrors />
					</Form.Field>
				{/each}

				<Form.Button class="w-full">Submit</Form.Button>
			</form>

			<div class="relative">
				<div class="absolute inset-0 flex items-center">
					<span class="w-full border-t border-dashed border-gray-200"></span>
				</div>
				<div class="relative flex justify-center text-xs uppercase">
					<span class="bg-elevated text-muted-foreground px-2"> Or </span>
				</div>
			</div>

			<Button
				data-sveltekit-preload-data="off"
				data-sveltekit-reload
				variant="outline"
				id="google-sign-in"
			>
				<IconBrandGoogle class="size-5" />continue with Google</Button
			>
			{#if page.params.auth == 'sign-up'}
				<span class="text-muted-foreground -mt-3 mb-1 text-center text-sm"
					>by continuing, you agree to our <a href="/" class="a">terms & conditions</a>
					and <a href="/" class="a">privacy policy</a>.</span
				>
			{/if}
		</div>
		<div class="relative mt-6 flex flex-col gap-2">
			<a class="a text-center text-sm" href="/sign-in">already have an account?</a>
		</div>
	</div>
</div>
