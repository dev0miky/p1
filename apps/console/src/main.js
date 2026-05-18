import { jsx as _jsx } from "react/jsx-runtime";
import ReactDOM from "react-dom/client";
function App() {
    return _jsx("main", { style: { fontFamily: "system-ui", padding: 24 }, children: "console" });
}
ReactDOM.createRoot(document.getElementById("root")).render(_jsx(App, {}));
