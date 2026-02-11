<script lang="ts" module>
	import AudioWaveformIcon from '@lucide/svelte/icons/audio-waveform';
	import ChartPieIcon from '@lucide/svelte/icons/chart-pie';
	import CommandIcon from '@lucide/svelte/icons/command';
	import FrameIcon from '@lucide/svelte/icons/frame';
	import GalleryVerticalEndIcon from '@lucide/svelte/icons/gallery-vertical-end';
	import MapIcon from '@lucide/svelte/icons/map';

	import { IconLogs } from '@tabler/icons-svelte';

	const data = {
		user: {
			name: 'shadcn',
			email: 'm@example.com',
			avatar: '/avatars/shadcn.jpg'
		},
		teams: [
			{
				name: 'Acme Inc',
				logo: GalleryVerticalEndIcon,
				plan: 'Enterprise'
			},
			{
				name: 'Acme Corp.',
				logo: AudioWaveformIcon,
				plan: 'Startup'
			},
			{
				name: 'Evil Corp.',
				logo: CommandIcon,
				plan: 'Free'
			}
		],
		navMain: [],
		projects: [
			{
				name: 'Design Engineering',
				url: '#',
				icon: FrameIcon
			},
			{
				name: 'Sales & Marketing',
				url: '#',
				icon: ChartPieIcon
			},
			{
				name: 'Travel',
				url: '#',
				icon: MapIcon
			}
		]
	};
</script>

<script lang="ts">
	import { page } from '$app/state';
	import * as Sidebar from '$lib/components/ui/sidebar/index.js';
	import type { ComponentProps } from 'svelte';
	import SidebarNavMain from './sidebar-nav-main.svelte';
	import SidebarNavUser from './sidebar-nav-user.svelte';
	import SidebarTeamSwitcher from './sidebar-team-switcher.svelte';
	let {
		ref = $bindable(null),
		collapsible = 'icon',
		...restProps
	}: ComponentProps<typeof Sidebar.Root> = $props();
</script>

<Sidebar.Root {collapsible} {...restProps}>
	<Sidebar.Header>
		<SidebarTeamSwitcher teams={data.teams} />
	</Sidebar.Header>
	<Sidebar.Content>
		<SidebarNavMain
			items={[
				{
					title: 'Logs',
					url: '/logs',
					icon: IconLogs,
					isActive: page.url.pathname.includes('/logs'),
					items: [
						{ title: 'All Logs', url: '/logs' },
						{ title: 'Saved Views', url: '/logs/views' },
						{ title: 'Sources', url: '/logs/sources' }
					]
				}
			]}
		/>
	</Sidebar.Content>
	<Sidebar.Footer>
		<SidebarNavUser user={data.user} />
	</Sidebar.Footer>
	<Sidebar.Rail />
</Sidebar.Root>
