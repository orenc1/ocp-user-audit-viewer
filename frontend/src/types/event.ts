export interface AuditEvent {
  id: string;
  audit_id: string;
  timestamp: string;
  username: string;
  user_groups: string[];
  verb: string;
  resource: string;
  subresource: string;
  namespace: string;
  name: string;
  api_group: string;
  api_version: string;
  request_uri: string;
  source_ips: string[];
  user_agent: string;
  response_code: number;
  request_object?: Record<string, unknown>;
  response_object?: Record<string, unknown>;
  annotations?: Record<string, string>;
}

export interface EventsResponse {
  events: AuditEvent[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

export interface EventQuery {
  username?: string;
  verb?: string;
  resource?: string;
  namespace?: string;
  name?: string;
  from?: string;
  to?: string;
  response_code?: number;
  client?: string;
  exclude_resource?: string;
  page?: number;
  page_size?: number;
  sort?: string;
}

export interface StatEntry {
  label: string;
  count: number;
}

export interface StatsResponse {
  top_users: StatEntry[];
  top_resources: StatEntry[];
  verb_breakdown: StatEntry[];
  total_events: number;
  db_size_bytes: number;
}
