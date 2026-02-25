import { JsonView, allExpanded, defaultStyles } from "react-json-view-lite";
import "react-json-view-lite/dist/index.css";

interface JsonViewerProps {
  data: unknown;
  title: string;
}

export default function JsonViewer({ data, title }: JsonViewerProps) {
  if (!data || (typeof data === "object" && Object.keys(data as object).length === 0)) {
    return null;
  }

  return (
    <div style={{ marginBottom: 16 }}>
      <h4 style={{ marginBottom: 8 }}>{title}</h4>
      <div
        style={{
          maxHeight: 400,
          overflow: "auto",
          backgroundColor: "#f5f5f5",
          borderRadius: 6,
          padding: 12,
        }}
      >
        <JsonView
          data={data as object}
          shouldExpandNode={allExpanded}
          style={defaultStyles}
        />
      </div>
    </div>
  );
}
