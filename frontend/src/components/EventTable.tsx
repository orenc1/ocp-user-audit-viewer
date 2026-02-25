import { Table, Tag, Typography } from "antd";
import type { ColumnsType, TablePaginationConfig } from "antd/es/table";
import type { AuditEvent, EventsResponse } from "../types/event";
import dayjs from "dayjs";

const { Text } = Typography;

const verbColors: Record<string, string> = {
  create: "green",
  update: "blue",
  patch: "geekblue",
  delete: "red",
  get: "default",
  list: "default",
};

interface EventTableProps {
  data: EventsResponse | null;
  loading: boolean;
  onPageChange: (page: number, pageSize: number) => void;
  onRowClick: (event: AuditEvent) => void;
}

export default function EventTable({ data, loading, onPageChange, onRowClick }: EventTableProps) {
  const columns: ColumnsType<AuditEvent> = [
    {
      title: "Timestamp",
      dataIndex: "timestamp",
      key: "timestamp",
      width: 190,
      render: (ts: string) => (
        <Text style={{ fontSize: 12 }}>{dayjs(ts).format("YYYY-MM-DD HH:mm:ss")}</Text>
      ),
    },
    {
      title: "User",
      dataIndex: "username",
      key: "username",
      width: 180,
      ellipsis: true,
    },
    {
      title: "Verb",
      dataIndex: "verb",
      key: "verb",
      width: 90,
      render: (verb: string) => <Tag color={verbColors[verb] || "default"}>{verb}</Tag>,
    },
    {
      title: "Namespace",
      dataIndex: "namespace",
      key: "namespace",
      width: 150,
      ellipsis: true,
      render: (ns: string) => ns || <Text type="secondary">-</Text>,
    },
    {
      title: "Resource",
      key: "resource",
      width: 200,
      render: (_: unknown, record: AuditEvent) => {
        const parts = [record.resource];
        if (record.subresource) parts.push(record.subresource);
        const full = parts.join("/");
        return record.name ? `${full}/${record.name}` : full;
      },
    },
    {
      title: "Status",
      dataIndex: "response_code",
      key: "response_code",
      width: 80,
      render: (code: number) => {
        const color = code >= 400 ? "red" : code >= 300 ? "orange" : "green";
        return <Tag color={color}>{code}</Tag>;
      },
    },
    {
      title: "Client",
      dataIndex: "user_agent",
      key: "user_agent",
      width: 100,
      render: (ua: string) => {
        if (!ua) return <Text type="secondary">-</Text>;
        if (ua.includes("oc/")) return <Tag color="blue">oc CLI</Tag>;
        if (ua.includes("kubectl/")) return <Tag color="cyan">kubectl</Tag>;
        if (ua.includes("Mozilla") || ua.includes("Chrome") || ua.includes("Safari"))
          return <Tag color="purple">Browser</Tag>;
        if (ua.includes("openshift")) return <Tag color="orange">Console</Tag>;
        return <Tag>{ua.split("/")[0]}</Tag>;
      },
    },
  ];

  const pagination: TablePaginationConfig = {
    current: data?.page || 1,
    pageSize: data?.page_size || 50,
    total: data?.total || 0,
    showSizeChanger: true,
    showTotal: (total) => `Total ${total} events`,
    pageSizeOptions: ["25", "50", "100", "200"],
    onChange: onPageChange,
  };

  return (
    <Table<AuditEvent>
      columns={columns}
      dataSource={data?.events || []}
      rowKey="id"
      loading={loading}
      pagination={pagination}
      size="small"
      scroll={{ x: 1000 }}
      onRow={(record) => ({
        onClick: () => onRowClick(record),
        style: { cursor: "pointer" },
      })}
    />
  );
}
