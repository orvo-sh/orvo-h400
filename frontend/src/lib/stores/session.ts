import { writable } from 'svelte/store';
import type { GetSessionOutputBody } from '$lib/api/model';

export const sessionStore = writable<GetSessionOutputBody | null>(null);
