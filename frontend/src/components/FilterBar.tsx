import { Button, Col, DatePicker, Input, Row, Select } from "antd";
import { SearchOutlined, ClearOutlined } from "@ant-design/icons";
import type { EventQuery } from "../types/event";
import dayjs from "dayjs";

const { RangePicker } = DatePicker;

const VERB_OPTIONS = [
  { label: "create", value: "create" },
  { label: "update", value: "update" },
  { label: "patch", value: "patch" },
  { label: "delete", value: "delete" },
  { label: "get", value: "get" },
  { label: "list", value: "list" },
];

const CLIENT_OPTIONS = [
  { label: "oc CLI", value: "oc/" },
  { label: "kubectl", value: "kubectl/" },
  { label: "Browser", value: "Mozilla" },
  { label: "OpenShift Console", value: "openshift" },
];

interface FilterBarProps {
  query: EventQuery;
  onChange: (query: EventQuery) => void;
  onSearch: () => void;
  loading: boolean;
}

export default function FilterBar({ query, onChange, onSearch, loading }: FilterBarProps) {
  const update = (partial: Partial<EventQuery>) => {
    onChange({ ...query, ...partial, page: 1 });
  };

  const handleClear = () => {
    onChange({ page: 1, page_size: query.page_size || 50 });
    onSearch();
  };

  return (
    <div style={{ marginBottom: 16 }}>
      <Row gutter={[12, 12]}>
        <Col xs={24} sm={12} md={6}>
          <Input
            placeholder="Username"
            value={query.username || ""}
            onChange={(e) => update({ username: e.target.value })}
            onPressEnter={onSearch}
            allowClear
          />
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Select
            mode="multiple"
            placeholder="Verb"
            style={{ width: "100%" }}
            value={query.verb ? query.verb.split(",") : []}
            onChange={(vals: string[]) => update({ verb: vals.join(",") })}
            options={VERB_OPTIONS}
            allowClear
          />
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Input
            placeholder="Resource (e.g. pods)"
            value={query.resource || ""}
            onChange={(e) => update({ resource: e.target.value })}
            onPressEnter={onSearch}
            allowClear
          />
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Input
            placeholder="Namespace"
            value={query.namespace || ""}
            onChange={(e) => update({ namespace: e.target.value })}
            onPressEnter={onSearch}
            allowClear
          />
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Input
            placeholder="Resource name"
            value={query.name || ""}
            onChange={(e) => update({ name: e.target.value })}
            onPressEnter={onSearch}
            allowClear
          />
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Input
            placeholder="Response code"
            type="number"
            value={query.response_code || ""}
            onChange={(e) =>
              update({ response_code: e.target.value ? parseInt(e.target.value) : undefined })
            }
            onPressEnter={onSearch}
            allowClear
          />
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Select
            mode="tags"
            placeholder="Client (e.g. oc CLI)"
            style={{ width: "100%" }}
            value={query.client ? query.client.split(",") : []}
            onChange={(vals: string[]) => update({ client: vals.join(",") })}
            options={CLIENT_OPTIONS}
            allowClear
          />
        </Col>
        <Col xs={24} sm={12} md={6}>
          <Select
            mode="tags"
            placeholder="Exclude resources (e.g. events)"
            style={{ width: "100%" }}
            value={query.exclude_resource ? query.exclude_resource.split(",") : []}
            onChange={(vals: string[]) => update({ exclude_resource: vals.join(",") })}
            allowClear
          />
        </Col>
        <Col xs={24} sm={12} md={6}>
          <RangePicker
            style={{ width: "100%" }}
            showTime
            value={
              query.from && query.to
                ? [dayjs(query.from), dayjs(query.to)]
                : null
            }
            onChange={(dates) => {
              if (dates && dates[0] && dates[1]) {
                update({
                  from: dates[0].toISOString(),
                  to: dates[1].toISOString(),
                });
              } else {
                update({ from: undefined, to: undefined });
              }
            }}
          />
        </Col>
        <Col xs={24} sm={12} md={6} style={{ display: "flex", gap: 8 }}>
          <Button type="primary" icon={<SearchOutlined />} onClick={onSearch} loading={loading}>
            Search
          </Button>
          <Button icon={<ClearOutlined />} onClick={handleClear}>
            Clear
          </Button>
        </Col>
      </Row>
    </div>
  );
}
