export type LogLevel = 'debug' | 'info' | 'warn' | 'error' | 'fatal';

export interface LogEntry {
	id: string;
	timestamp: string;
	level: LogLevel;
	source: string;
	service: string;
	host: string;
	message: string;
	group?: string;
	tags?: string[];
	metadata?: Record<string, string | number | boolean>;
}

// Generate mock log data
const services = [
	'api-gateway',
	'user-service',
	'payment-service',
	'notification-service',
	'auth-service',
	'frontend'
];
const hosts = ['prod-us-east-1a', 'prod-us-west-2b', 'prod-eu-west-1c', 'staging-1', 'dev-local'];
const groups = ['nightman', 'background-jobs', 'api-requests', 'system', 'cron'];
const sources = ['kubernetes', 'docker', 'systemd', 'application', 'nginx'];

const logMessages: Record<LogLevel, string[]> = {
	debug: [
		'Cache hit for key: user_session_12345',
		'Database connection pool stats: active=5, idle=15, waiting=0',
		'Request headers: Content-Type=application/json, Accept=*/*',
		'Starting background job: cleanup_old_sessions',
		'Memory usage: heap_used=156MB, heap_total=256MB'
	],
	info: [
		"Operation: 'process_batched_emails' took 0.24937302185 minutes",
		'User login successful for user_id: 42851',
		'API request completed: GET /api/v1/users/profile (200, 45ms)',
		'New WebSocket connection established from 192.168.1.105',
		'Deployment v2.4.1 successfully rolled out to production',
		'Scheduled task completed: daily_report_generation'
	],
	warn: [
		'Rate limit threshold reached: 850/1000 requests for IP 10.0.0.55',
		'Deprecated API endpoint called: POST /api/v1/legacy/auth',
		'Database query took longer than expected: 2.5s (threshold: 1s)',
		'SSL certificate expires in 14 days for domain api.example.com',
		'Memory usage above 80%: current=82%, threshold=80%'
	],
	error: [
		'Failed to connect to Redis cluster: ECONNREFUSED 127.0.0.1:6379',
		"Database query failed: relation 'user_sessions' does not exist",
		'Payment processing failed: Card declined for order #89234',
		'Email delivery failed: SMTP connection timeout after 30s',
		'File upload failed: Maximum file size exceeded (limit: 10MB)'
	],
	fatal: [
		'CRITICAL: Database cluster unreachable - all replicas down',
		'Out of memory: Cannot allocate 512MB for process worker-3',
		'FATAL: SSL handshake failed - certificate chain broken',
		'System crash: Kernel panic - not syncing: VFS: Unable to mount root fs'
	]
};

function randomDate(hoursAgo: number = 2): string {
	const now = new Date();
	const msAgo = Math.random() * hoursAgo * 60 * 60 * 1000;
	const date = new Date(now.getTime() - msAgo);
	return date.toISOString();
}

function randomChoice<T>(arr: T[]): T {
	return arr[Math.floor(Math.random() * arr.length)];
}

function generateLogEntry(id: number): LogEntry {
	// Weight towards info logs, fewer errors/fatals
	const levelWeights: [LogLevel, number][] = [
		['debug', 0.15],
		['info', 0.5],
		['warn', 0.2],
		['error', 0.12],
		['fatal', 0.03]
	];

	let rand = Math.random();
	let level: LogLevel = 'info';
	for (const [l, weight] of levelWeights) {
		if (rand < weight) {
			level = l;
			break;
		}
		rand -= weight;
	}

	const service = randomChoice(services);
	const host = randomChoice(hosts);

	return {
		id: `log-${id}`,
		timestamp: randomDate(2),
		level,
		source: randomChoice(sources),
		service,
		host,
		message: randomChoice(logMessages[level]),
		group: Math.random() > 0.3 ? randomChoice(groups) : undefined,
		tags:
			Math.random() > 0.5
				? [randomChoice(['slow_query', 'http', 'database', 'cache', 'security', 'performance'])]
				: undefined,
		metadata: {
			request_id: `req-${Math.random().toString(36).substr(2, 9)}`,
			duration_ms: Math.floor(Math.random() * 500),
			...(Math.random() > 0.5 ? { user_id: Math.floor(Math.random() * 100000) } : {})
		}
	};
}

// Generate 100 mock log entries
export const mockLogs: LogEntry[] = Array.from({ length: 100 }, (_, i) => generateLogEntry(i)).sort(
	(a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
);

// Generate histogram data for the chart (last 2 hours, 5-minute buckets)
export interface HistogramBucket {
	time: Date;
	count: number;
	debug: number;
	info: number;
	warn: number;
	error: number;
	fatal: number;
}

export function generateHistogramData(): HistogramBucket[] {
	const buckets: HistogramBucket[] = [];
	const now = new Date();
	const bucketSize = 5 * 60 * 1000; // 5 minutes
	const numBuckets = 24; // 2 hours of data

	for (let i = numBuckets - 1; i >= 0; i--) {
		const time = new Date(now.getTime() - i * bucketSize);
		const debug = Math.floor(Math.random() * 10);
		const info = Math.floor(Math.random() * 40) + 10;
		const warn = Math.floor(Math.random() * 15);
		const error = Math.floor(Math.random() * 8);
		const fatal = Math.floor(Math.random() * 2);

		buckets.push({
			time,
			count: debug + info + warn + error + fatal,
			debug,
			info,
			warn,
			error,
			fatal
		});
	}

	return buckets;
}

// Saved views mock data
export interface SavedView {
	id: string;
	name: string;
	query: string;
	filters: {
		services?: string[];
		levels?: LogLevel[];
		hosts?: string[];
	};
}

export const savedViews: SavedView[] = [
	{
		id: 'view-1',
		name: 'Production Errors',
		query: 'level:error OR level:fatal',
		filters: {
			levels: ['error', 'fatal'],
			hosts: ['prod-us-east-1a', 'prod-us-west-2b', 'prod-eu-west-1c']
		}
	},
	{
		id: 'view-2',
		name: 'API Gateway Logs',
		query: 'service:api-gateway',
		filters: {
			services: ['api-gateway']
		}
	},
	{
		id: 'view-3',
		name: 'Slow Queries',
		query: 'tag:slow_query',
		filters: {}
	}
];

// Log sources
export interface LogSource {
	id: string;
	name: string;
	type: string;
	status: 'active' | 'inactive' | 'error';
	logsPerMinute: number;
}

export const logSources: LogSource[] = [
	{
		id: 'src-1',
		name: 'Production Kubernetes',
		type: 'kubernetes',
		status: 'active',
		logsPerMinute: 1250
	},
	{
		id: 'src-2',
		name: 'Staging Environment',
		type: 'kubernetes',
		status: 'active',
		logsPerMinute: 340
	},
	{
		id: 'src-3',
		name: 'Legacy Docker Stack',
		type: 'docker',
		status: 'inactive',
		logsPerMinute: 0
	},
	{ id: 'src-4', name: 'Development Local', type: 'docker', status: 'active', logsPerMinute: 45 },
	{ id: 'src-5', name: 'CI/CD Pipeline', type: 'systemd', status: 'error', logsPerMinute: 0 }
];
