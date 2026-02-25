import { useCallback, useEffect, useState } from "react";
import { Card, message, Modal, Descriptions, Tag, Button, Space } from "antd";
import { DownloadOutlined } from "@ant-design/icons";
import FilterBar from "../components/FilterBar";
import EventTable from "../components/EventTable";
import JsonViewer from "../components/JsonViewer";
import { fetchEvents } from "../api/client";
import type { AuditEvent, EventQuery, EventsResponse } from "../types/event";
import dayjs from "dayjs";

export default function EventExplorer() {
  const [query, setQuery] = useState<EventQuery>({ page: 1, page_size: 50 });
  const [data, setData] = useState<EventsResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [selectedEvent, setSelectedEvent] = useState<AuditEvent | null>(null);

  const doSearch = useCallback(async () => {
    setLoading(true);
    try {
      const result = await fetchEvents(query);
      setData(result);
    } catch (err) {
      message.error("Failed to fetch events");
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [query]);

  useEffect(() => {
    doSearch();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handlePageChange = (page: number, pageSize: number) => {
    const newQuery = { ...query, page, page_size: pageSize };
    setQuery(newQuery);
    setLoading(true);
    fetchEvents(newQuery)
      .then(setData)
      .catch(() => message.error("Failed to fetch events"))
      .finally(() => setLoading(false));
  };

  const handleExportCSV = () => {
    if (!data || data.events.length === 0) return;

    const headers = [
      "timestamp", "username", "verb", "namespace", "resource", "name",
      "response_code", "source_ips", "request_uri",
    ];
    const rows = data.events.map((e) => [
      e.timestamp,
      e.username,
      e.verb,
      e.namespace,
      e.resource + (e.subresource ? `/${e.subresource}` : ""),
      e.name,
      e.response_code,
      (e.source_ips || []).join(";"),
      e.request_uri,
    ]);

    const csv = [headers.join(","), ...rows.map((r) => r.map((v) => `"${v}"`).join(","))].join("\n");
    const blob = new Blob([csv], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `audit-events-${dayjs().format("YYYY-MM-DD-HHmmss")}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div>
      <Card style={{ marginBottom: 16 }}>
        <FilterBar query={query} onChange={setQuery} onSearch={doSearch} loading={loading} />
      </Card>

      <Card
        title="Audit Events"
        extra={
          <Space>
            <Button icon={<DownloadOutlined />} onClick={handleExportCSV} disabled={!data?.events.length}>
              Export CSV
            </Button>
          </Space>
        }
      >
        <EventTable data={data} loading={loading} onPageChange={handlePageChange} onRowClick={setSelectedEvent} />
      </Card>

      <Modal
        title="Event Details"
        open={!!selectedEvent}
        onCancel={() => setSelectedEvent(null)}
        footer={null}
        width={800}
      >
        {selectedEvent && (
          <div>
            <Descriptions bordered size="small" column={2} style={{ marginBottom: 16 }}>
              <Descriptions.Item label="Audit ID">{selectedEvent.audit_id}</Descriptions.Item>
              <Descriptions.Item label="Timestamp">
                {dayjs(selectedEvent.timestamp).format("YYYY-MM-DD HH:mm:ss.SSS")}
              </Descriptions.Item>
              <Descriptions.Item label="Username">{selectedEvent.username}</Descriptions.Item>
              <Descriptions.Item label="Verb">
                <Tag>{selectedEvent.verb}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Resource">
                {selectedEvent.api_group ? `${selectedEvent.api_group}/` : ""}
                {selectedEvent.api_version}/{selectedEvent.resource}
                {selectedEvent.subresource ? `/${selectedEvent.subresource}` : ""}
              </Descriptions.Item>
              <Descriptions.Item label="Name">{selectedEvent.name || "-"}</Descriptions.Item>
              <Descriptions.Item label="Namespace">{selectedEvent.namespace || "-"}</Descriptions.Item>
              <Descriptions.Item label="Response Code">
                <Tag color={selectedEvent.response_code >= 400 ? "red" : "green"}>
                  {selectedEvent.response_code}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="Source IPs" span={2}>
                {(selectedEvent.source_ips || []).join(", ") || "-"}
              </Descriptions.Item>
              <Descriptions.Item label="User Agent" span={2}>
                {selectedEvent.user_agent || "-"}
              </Descriptions.Item>
              <Descriptions.Item label="Request URI" span={2}>
                <code style={{ wordBreak: "break-all" }}>{selectedEvent.request_uri}</code>
              </Descriptions.Item>
            </Descriptions>

            <JsonViewer data={selectedEvent.request_object} title="Request Object (Change)" />
            <JsonViewer data={selectedEvent.response_object} title="Response Object" />
            <JsonViewer data={selectedEvent.annotations} title="Annotations" />
          </div>
        )}
      </Modal>
    </div>
  );
}
