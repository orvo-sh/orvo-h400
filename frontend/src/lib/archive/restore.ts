import type { ErrorModel } from '$lib/api/model';

const API_BASE = 'http://localhost:8080/api/v1';

export type TelemetrySignal = 'logs' | 'traces' | 'metrics';

export interface RestoreRequiredInfo {
	signal: TelemetrySignal;
	startDay: string;
	endDay: string;
	missingDays: string[];
	restorableDays: string[];
	jobId?: string;
	jobState?: string;
}

export interface RestoreJobItem {
	id: string;
	day: string;
	object_key: string;
	state: string;
	object_size_bytes: number;
	restored_rows: number;
	error?: string;
}

export interface RestoreJob {
	id: string;
	organization_id: string;
	signal: string;
	start_day: string;
	end_day: string;
	state: string;
	total_items: number;
	completed_items: number;
	total_bytes: number;
	done_bytes: number;
	estimated_seconds: number;
	error?: string;
	items?: RestoreJobItem[];
}

export function parseRestoreRequired(error?: ErrorModel): RestoreRequiredInfo | null {
	if (!error || error.detail !== 'restore_required') {
		return null;
	}

	const details = error.errors ?? [];
	const signal = stringDetail(details, 'restore.signal') as TelemetrySignal | undefined;
	const startDay = stringDetail(details, 'restore.start_day');
	const endDay = stringDetail(details, 'restore.end_day');
	const missingDays = arrayDetail(details, 'restore.missing_days');
	const restorableDays = arrayDetail(details, 'restore.restorable_days');
	const jobId = stringDetail(details, 'restore.job_id');
	const jobState = stringDetail(details, 'restore.job_state');

	if (!signal || !startDay || !endDay) {
		return null;
	}

	return {
		signal,
		startDay,
		endDay,
		missingDays,
		restorableDays,
		jobId,
		jobState
	};
}

export async function createRestoreJob(
	organizationID: string,
	signal: TelemetrySignal,
	startDay: string,
	endDay: string
): Promise<RestoreJob> {
	const response = await fetch(`${API_BASE}/organizations/${organizationID}/archive/restores`, {
		method: 'POST',
		credentials: 'include',
		headers: {
			'Content-Type': 'application/json'
		},
		body: JSON.stringify({
			signal,
			start_day: startDay,
			end_day: endDay
		})
	});

	const payload = await response.json().catch(() => ({}));
	if (!response.ok) {
		const detail = payload?.detail ?? `failed to create restore job (${response.status})`;
		throw new Error(detail);
	}
	return payload as RestoreJob;
}

export async function getRestoreJob(
	organizationID: string,
	restoreJobID: string
): Promise<RestoreJob> {
	const response = await fetch(
		`${API_BASE}/organizations/${organizationID}/archive/restores/${restoreJobID}`,
		{
			method: 'GET',
			credentials: 'include'
		}
	);

	const payload = await response.json().catch(() => ({}));
	if (!response.ok) {
		const detail = payload?.detail ?? `failed to fetch restore job (${response.status})`;
		throw new Error(detail);
	}
	return payload as RestoreJob;
}

function stringDetail(
	details: NonNullable<ErrorModel['errors']>,
	location: string
): string | undefined {
	const value = details.find((detail) => detail.location === location)?.value;
	return typeof value === 'string' ? value : undefined;
}

function arrayDetail(details: NonNullable<ErrorModel['errors']>, location: string): string[] {
	const value = details.find((detail) => detail.location === location)?.value;
	if (!Array.isArray(value)) {
		return [];
	}
	return value.filter((item): item is string => typeof item === 'string');
}
