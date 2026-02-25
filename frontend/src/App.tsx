import { ConfigProvider, Layout, Typography, theme } from "antd";
import { AuditOutlined } from "@ant-design/icons";
import EventExplorer from "./pages/EventExplorer";

const { Header, Content } = Layout;
const { Title } = Typography;

function App() {
  return (
    <ConfigProvider
      theme={{
        algorithm: theme.defaultAlgorithm,
        token: {
          colorPrimary: "#1677ff",
          borderRadius: 6,
        },
      }}
    >
      <Layout style={{ minHeight: "100vh" }}>
        <Header
          style={{
            display: "flex",
            alignItems: "center",
            gap: 12,
            background: "#001529",
            padding: "0 24px",
          }}
        >
          <AuditOutlined style={{ fontSize: 24, color: "#fff" }} />
          <Title level={4} style={{ color: "#fff", margin: 0 }}>
            OpenShift User Audit Viewer
          </Title>
        </Header>
        <Content style={{ padding: 24, background: "#f0f2f5" }}>
          <EventExplorer />
        </Content>
      </Layout>
    </ConfigProvider>
  );
}

export default App;
