<script lang="ts">
	import { z } from 'zod';

	import { IconBrandGoogle } from '@tabler/icons-svelte';

	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Logo } from '$lib/components/ui/logo';
	import { register } from '$lib/api/endpoints/auth/auth';

	const signUpSchema = z.object({
		name: z.string().min(2).max(32),
		email: z.string().email(),
		password: z.string().min(8)
	});

	let name = $state('');
	let email = $state('');
	let password = $state('');
	let error = $state('');
	let submitting = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		error = '';

		const parsed = signUpSchema.safeParse({
			name,
			email,
			password
		});
		if (!parsed.success) {
			error = parsed.error.issues[0]?.message ?? 'Enter valid account details';
			return;
		}

		submitting = true;
		try {
			const res = await register(parsed.data);
			if (res.status === 204) {
				await goto('/logs');
				return;
			}
			error = (res.data as any)?.detail ?? 'Registration failed';
		} catch {
			error = 'An unexpected error occurred';
		} finally {
			submitting = false;
		}
	}
</script>

<div
	class="sm:flex-center border-primary relative h-screen min-h-screen flex-row gap-6 overflow-hidden p-0 pt-[10%] sm:pt-0 md:p-6 lg:gap-16"
>
	<div class="flex-center z-10 flex flex-col p-3 sm:p-4">
		<div class="text-foreground mb-4 flex items-center gap-2 text-xl font-bold tracking-tight">
			<Logo class="h-10 w-10" />
			<span class="mb-0.5 lg:block"> Orvo </span>
		</div>

		<div
			class="bg-elevated flex w-full flex-col gap-6   px-4 py-4 sm:w-[28rem] sm:max-w-[28rem]"
		>
			<div class="flex flex-col items-center space-y-0 text-center">
				<h1 class="text-foreground text-xl font-semibold tracking-tight">
					Get started with Orvo
				</h1>
				<p class="text-muted-foreground text-sm">continue with your email address or Google.</p>
			</div>

			{#if error}
				<div class="text-destructive bg-destructive/10 rounded-md px-3 py-2 text-sm">{error}</div>
			{/if}

			<form method="POST" onsubmit={handleSubmit} class="space-y-4">
				<div class="space-y-2">
					<label class="text-sm font-medium" for="name">Name</label>
					<Input id="name" bind:value={name} type="text" />
				</div>

				<div class="space-y-2">
					<label class="text-sm font-medium" for="email">Email</label>
					<Input id="email" bind:value={email} type="email" />
				</div>

				<div class="space-y-2">
					<label class="text-sm font-medium" for="password">Password</label>
					<Input id="password" bind:value={password} type="password" />
				</div>

				<Button class="w-full" type="submit" disabled={submitting}>
					{#if submitting}
						Signing up...
					{:else}
						Sign up
					{/if}
				</Button>
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
			<span class="text-muted-foreground -mt-3 mb-1 text-center text-sm"
				>by continuing, you agree to our <a href="/" class="a">terms & conditions</a>
				and <a href="/" class="a">privacy policy</a>.</span
			>
		</div>
		<div class="relative mt-6 flex flex-col gap-2">
			<a class="a text-center text-sm" href="/sign-in">already have an account?</a>
		</div>
	</div>
</div>
