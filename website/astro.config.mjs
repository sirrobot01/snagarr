// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
	site: 'https://sirrobot01.github.io',
	base: '/snagarr',
	integrations: [
		starlight({
			title: 'Snagarr',
			description:
				'Snagarr captures a film or show, resolves it against TMDB, sends it to Radarr or Sonarr, and puts it in a collection on your media server when the file lands.',
			social: [
				{
					icon: 'github',
					label: 'GitHub',
					href: 'https://github.com/sirrobot01/snagarr',
				},
			],
			editLink: {
				baseUrl: 'https://github.com/sirrobot01/snagarr/edit/main/website/',
			},
			customCss: [
				'@fontsource/archivo/400.css',
				'@fontsource/archivo/600.css',
				'@fontsource/archivo/700.css',
				'./src/styles/custom.css',
			],
			components: {
				// Defaults to dark instead of following the operating system.
				ThemeProvider: './src/components/ThemeProvider.astro',
			},
			sidebar: [
				{
					label: 'Start',
					items: [
						{ label: 'Introduction', link: '/' },
						{ label: 'Install', slug: 'start/install' },
						{ label: 'First run', slug: 'start/first-run' },
					],
				},
				{
					label: 'Configure',
					items: [
						{ label: 'Services', slug: 'configure/services' },
						{ label: 'Settings', slug: 'configure/settings' },
						{ label: 'Environment variables', slug: 'configure/environment' },
						{ label: 'Media servers', slug: 'configure/media-servers' },
						{ label: 'Radarr and Sonarr', slug: 'configure/radarr-sonarr' },
						{ label: 'Notifications', slug: 'configure/notifications' },
					],
				},
				{
					label: 'Use',
					items: [
						{ label: 'Capture clients', slug: 'use/clients' },
						{ label: 'Webhooks', slug: 'use/webhooks' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ label: 'HTTP API', slug: 'reference/api' },
						{ label: 'Troubleshooting', slug: 'reference/troubleshooting' },
					],
				},
			],
		}),
	],
});
