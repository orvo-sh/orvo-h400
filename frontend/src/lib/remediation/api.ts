import {
	deleteServiceRemediationMapping as deleteServiceRemediationMappingRequest,
	getLogAutoResolvePreview as getLogAutoResolvePreviewRequest,
	listServiceRemediationMappings as listServiceRemediationMappingsRequest,
	runLogAutoResolve as runLogAutoResolveRequest,
	upsertServiceRemediationMapping as upsertServiceRemediationMappingRequest
} from '$lib/api/endpoints/remediation/remediation';
import type {
	AutoResolvePreview,
	ErrorDetail,
	ErrorModel,
	RunLogAutoResolveOutputBody,
	ServiceRemediationMapping
} from '$lib/api/model';

export type { AutoResolvePreview, ServiceRemediationMapping } from '$lib/api/model';

export interface APIErrorDetail {
	location?: string;
	message?: string;
	value?: unknown;
}

export class RemediationAPIError extends Error {
	status: number;
	code: string;
	details: APIErrorDetail[];

	constructor(status: number, code: string, details: APIErrorDetail[] = [], message?: string) {
		super(code);
		this.status = status;
		this.code = code;
		this.details = details;
		if (message) {
			this.message = message;
		}
	}
}

function toDetails(value: unknown): APIErrorDetail[] {
	if (!Array.isArray(value)) return [];
	return value.filter((item): item is ErrorDetail => typeof item === 'object' && item !== null);
}

function toRemediationError(status: number, data: unknown): RemediationAPIError {
	const payload = (data ?? {}) as ErrorModel;
	const code = payload?.detail ?? `request_failed_${status}`;
	return new RemediationAPIError(status, code, toDetails(payload?.errors));
}

function wrapEndpointError(error: unknown): never {
	if (error instanceof RemediationAPIError) {
		throw error;
	}
	if (error instanceof SyntaxError) {
		throw new RemediationAPIError(
			502,
			'invalid_json_response',
			[],
			'invalid response from server'
		);
	}
	if (error instanceof Error) {
		throw new RemediationAPIError(500, 'request_failed', [], error.message);
	}

	throw new RemediationAPIError(500, 'request_failed');
}

export async function listServiceRemediationMappings(
	organizationID: string
): Promise<ServiceRemediationMapping[]> {
	try {
		const response = await listServiceRemediationMappingsRequest(organizationID);
		if (response.status !== 200) {
			throw toRemediationError(response.status, response.data);
		}
		return response.data.mappings ?? [];
	} catch (error) {
		return wrapEndpointError(error);
	}
}

export async function upsertServiceRemediationMapping(
	organizationID: string,
	serviceName: string,
	repositoryID: string
): Promise<ServiceRemediationMapping> {
	try {
		const response = await upsertServiceRemediationMappingRequest(
			organizationID,
			encodeURIComponent(serviceName),
			{
				repository_id: repositoryID
			}
		);
		if (response.status !== 200) {
			throw toRemediationError(response.status, response.data);
		}
		return response.data;
	} catch (error) {
		return wrapEndpointError(error);
	}
}

export async function deleteServiceRemediationMapping(
	organizationID: string,
	serviceName: string
): Promise<void> {
	try {
		const response = await deleteServiceRemediationMappingRequest(
			organizationID,
			encodeURIComponent(serviceName)
		);
		if (response.status !== 204) {
			throw toRemediationError(response.status, response.data);
		}
	} catch (error) {
		return wrapEndpointError(error);
	}
}

export async function getLogAutoResolvePreview(
	organizationID: string,
	logID: string
): Promise<AutoResolvePreview> {
	try {
		const response = await getLogAutoResolvePreviewRequest(organizationID, encodeURIComponent(logID));
		if (response.status !== 200) {
			throw toRemediationError(response.status, response.data);
		}
		return response.data;
	} catch (error) {
		return wrapEndpointError(error);
	}
}

export async function runLogAutoResolve(
	organizationID: string,
	logID: string
): Promise<RunLogAutoResolveOutputBody> {
	try {
		const response = await runLogAutoResolveRequest(organizationID, encodeURIComponent(logID));
		if (response.status !== 200) {
			throw toRemediationError(response.status, response.data);
		}
		return response.data;
	} catch (error) {
		return wrapEndpointError(error);
	}
}
