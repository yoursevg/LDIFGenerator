import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./style.css";

class ErrorBoundary extends React.Component<{ children: React.ReactNode }, { error: Error | null }> {
  state: { error: Error | null } = { error: null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  render() {
    if (this.state.error) {
      return (
        <main className="app">
          <section className="topbar">
            <div>
              <h1>LDIFGenerator</h1>
              <p>Ошибка интерфейса</p>
            </div>
          </section>
          <section className="crash-panel">
            <h2>Интерфейс не смог отрисоваться</h2>
            <pre>{this.state.error.message}</pre>
            <button onClick={() => window.location.reload()}>Перезагрузить</button>
          </section>
        </main>
      );
    }
    return this.props.children;
  }
}

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </React.StrictMode>,
);
