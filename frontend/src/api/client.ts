import axios from "axios";
import type { EventsResponse, EventQuery, AuditEvent, StatsResponse } from "../types/event";

const api = axios.create({
  baseURL: "/api/v1",
  timeout: 30000,
});

export async function fetchEvents(query: EventQuery): Promise<EventsResponse> {
  const params: Record<string, string | number> = {};
  if (query.username) params.username = query.username;
  if (query.verb) params.verb = query.verb;
  if (query.resource) params.resource = query.resource;
  if (query.namespace) params.namespace = query.namespace;
  if (query.name) params.name = query.name;
  if (query.from) params.from = query.from;
  if (query.to) params.to = query.to;
  if (query.response_code) params.response_code = query.response_code;
  if (query.client) params.client = query.client;
  if (query.exclude_resource) params.exclude_resource = query.exclude_resource;
  if (query.page) params.page = query.page;
  if (query.page_size) params.page_size = query.page_size;
  if (query.sort) params.sort = query.sort;

  const { data } = await api.get<EventsResponse>("/events", { params });
  return data;
}

export async function fetchEvent(id: string): Promise<AuditEvent> {
  const { data } = await api.get<AuditEvent>(`/events/${id}`);
  return data;
}

export async function fetchStats(): Promise<StatsResponse> {
  const { data } = await api.get<StatsResponse>("/stats");
  return data;
}
