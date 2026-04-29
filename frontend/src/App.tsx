import { FolderOpen, Play, Square, Upload } from "lucide-react";
import type { ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";
import { cancelGeneration, defaultConfig, generate, loadSchema, progress, selectOutputPath, selectSchemaFiles } from "./wails";
import type { AttributeType, EntryType, GeneratorConfig, ObjectClassSummary, Report, SchemaSummary } from "./types";

const entryTypes: { key: EntryType; label: string }[] = [
  { key: "user", label: "Пользователи" },
  { key: "privilegedUser", label: "Привилегированные" },
  { key: "group", label: "Группы" },
  { key: "computer", label: "Компьютеры" },
  { key: "serviceAccount", label: "Сервисные аккаунты" },
];

export default function App() {
  const [schemaPaths, setSchemaPaths] = useState("examples/schema.ldif");
  const [schema, setSchema] = useState<SchemaSummary | null>(null);
  const [config, setConfig] = useState<GeneratorConfig | null>(null);
  const [selectedOC, setSelectedOC] = useState<ObjectClassSummary | null>(null);
  const [selectedAttr, setSelectedAttr] = useState<AttributeType | null>(null);
  const [ocFilter, setOCFilter] = useState("");
  const [attrFilter, setAttrFilter] = useState("");
  const [status, setStatus] = useState("");
  const [running, setRunning] = useState(false);
  const [written, setWritten] = useState(0);
  const [report, setReport] = useState<Report | null>(null);

  useEffect(() => {
    defaultConfig().then((cfg) => setConfig(normalizeConfig(cfg))).catch((err) => setStatus(errorText(err)));
  }, []);

  useEffect(() => {
    if (!running) return;
    const id = window.setInterval(async () => {
      try {
        const p = await progress();
        setWritten(Number(p.written) || 0);
      } catch (err) {
        setStatus(errorText(err));
      }
    }, 350);
    return () => window.clearInterval(id);
  }, [running]);

  const objectClassNames = useMemo(() => (schema?.objectClasses ?? []).map((oc) => oc.name).filter(Boolean).sort(), [schema]);
  const filteredObjectClasses = useMemo(() => {
    const q = ocFilter.trim().toLowerCase();
    return [...(schema?.objectClasses ?? [])]
      .sort((a, b) => a.name.localeCompare(b.name))
      .filter((oc) => {
        if (!q) return true;
        return [oc.name, oc.oid, oc.kind, ...(oc.sup ?? []), ...(oc.must ?? []), ...(oc.may ?? [])]
          .filter(Boolean)
          .some((value) => String(value).toLowerCase().includes(q));
      });
  }, [schema, ocFilter]);
  const filteredAttributes = useMemo(() => {
    const q = attrFilter.trim().toLowerCase();
    return [...(schema?.attributeTypes ?? [])]
      .sort((a, b) => primaryAttrName(a).localeCompare(primaryAttrName(b)))
      .filter((attr) => {
        if (!q) return true;
        return [attr.oid, attr.sup, attr.syntax, ...(attr.names ?? [])]
          .filter(Boolean)
          .some((value) => String(value).toLowerCase().includes(q));
      });
  }, [schema, attrFilter]);
  const optionalAttributeNames = useMemo(() => {
    const names = new Set<string>();
    for (const oc of schema?.objectClasses ?? []) {
      for (const attr of oc.may ?? []) {
        names.add(attr);
      }
    }
    return [...names].sort((a, b) => a.localeCompare(b));
  }, [schema]);

  async function onLoadSchema() {
    try {
      setStatus("Разбор схемы...");
      const paths = schemaPaths.split(",").map((p) => p.trim()).filter(Boolean);
      if (paths.length === 0) {
        setStatus("Выберите хотя бы один файл схемы");
        return;
      }
      const loaded = normalizeSchema(await loadSchema(paths));
      setSchema(loaded);
      setSelectedOC(loaded.objectClasses[0] ?? null);
      setSelectedAttr(loaded.attributeTypes[0] ?? null);
      setStatus(`Загружено attributeTypes: ${loaded.attributeTypes.length}, objectClasses: ${loaded.objectClasses.length}`);
    } catch (err) {
      setStatus(errorText(err));
    }
  }

  async function onSelectSchemaFiles() {
    try {
      const paths = await selectSchemaFiles();
      if (Array.isArray(paths) && paths.length > 0) setSchemaPaths(paths.join(","));
    } catch (err) {
      setStatus(errorText(err));
    }
  }

  async function onSelectOutputPath() {
    try {
      const path = await selectOutputPath();
      if (path) updateConfig({ outputPath: path });
    } catch (err) {
      setStatus(errorText(err));
    }
  }

  async function onGenerate() {
    if (!config) return;
    const nextConfig = normalizeConfig(config);
    if (!nextConfig.outputPath.trim()) {
      setStatus("Укажите путь выходного LDIF-файла");
      return;
    }
    try {
      setRunning(true);
      setWritten(0);
      setReport(null);
      setStatus("Генерация LDIF...");
      const r = await generate(nextConfig);
      setReport(r);
      setWritten(nextConfig.count);
      setStatus("Генерация завершена");
    } catch (err) {
      setStatus(errorText(err));
    } finally {
      setRunning(false);
    }
  }

  async function onCancel() {
    try {
      await cancelGeneration();
      setStatus("Отмена запрошена");
    } catch (err) {
      setStatus(errorText(err));
    }
  }

  function updateConfig(patch: Partial<GeneratorConfig>) {
    setConfig((prev) => (prev ? normalizeConfig({ ...prev, ...patch }) : prev));
  }

  function updateRel(key: keyof GeneratorConfig["relationships"], value: number) {
    setConfig((prev) => (prev ? normalizeConfig({ ...prev, relationships: { ...prev.relationships, [key]: value } }) : prev));
  }

  function updateTreePercent(key: "privilegedPercent" | "groupPercent" | "computerPercent" | "servicePercent", value: number) {
    setConfig((prev) => (prev ? normalizeConfig({ ...prev, tree: { ...prev.tree, [key]: value } }) : prev));
  }

  function setObjectClass(type: EntryType, value: string) {
    setConfig((prev) => {
      if (!prev) return prev;
      return normalizeConfig({ ...prev, objectClasses: { ...prev.objectClasses, [type]: value ? [value] : [] } });
    });
  }

  function toggleOptionalAttribute(name: string) {
    setConfig((prev) => {
      if (!prev) return prev;
      const key = name.toLowerCase();
      const selectedAttributes = materializeSelectedAttributes(prev.selectedAttributes ?? {}, optionalAttributeNames);
      return normalizeConfig({
        ...prev,
        selectedAttributes: { ...selectedAttributes, [key]: !isAttributeEnabled(prev.selectedAttributes ?? {}, name) },
      });
    });
  }

  function setAllOptionalAttributes(enabled: boolean) {
    setConfig((prev) => {
      if (!prev) return prev;
      if (enabled) {
        return normalizeConfig({ ...prev, selectedAttributes: {} });
      }
      return normalizeConfig({ ...prev, selectedAttributes: materializeSelectedAttributes({}, optionalAttributeNames, false) });
    });
  }

  if (!config) return <div className="app">Загрузка...</div>;
  const percent = config.count > 0 ? Math.min(100, Math.round((written / config.count) * 100)) : 0;

  return (
    <main className="app">
      <section className="topbar">
        <div>
          <h1>LDIFGenerator</h1>
          <p>Генерация LDIF для нагрузочного тестирования LDAP</p>
        </div>
        <div className="actions">
          <button onClick={onGenerate} disabled={running || !schema} title="Запустить генерацию">
            <Play size={18} /> Сгенерировать
          </button>
          <button onClick={onCancel} disabled={!running} title="Отменить генерацию">
            <Square size={18} /> Отменить
          </button>
        </div>
      </section>

      <section className="statusbar">
        <div className="progress"><span style={{ width: `${percent}%` }} /></div>
        <strong>{percent}%</strong>
        <span>{status || "Готово к работе"}</span>
        {report && <span>{report.records} записей, {Math.round(report.recordsPerSec)} зап/сек, {formatBytes(report.fileBytes)}</span>}
      </section>

      <section className="workspace">
        <div className="panel schema-panel">
          <header><Upload size={18} /> Schema</header>
          <label>Schema files</label>
          <div className="inline">
            <input value={schemaPaths} onChange={(e) => setSchemaPaths(e.target.value)} />
            <button onClick={onSelectSchemaFiles} title="Выбрать schema files"><FolderOpen size={17} /></button>
            <button onClick={onLoadSchema}>Загрузить</button>
          </div>

          <div className="schema-stack">
            <div className="schema-list-block objectclass-block">
              <h3>ObjectClasses ({schema?.objectClasses.length ?? 0})</h3>
              <input className="filter-input" value={ocFilter} onChange={(e) => setOCFilter(e.target.value)} placeholder="Filter objectClass" />
              <div className="schema-list">
                {filteredObjectClasses.map((oc) => (
                  <button key={oc.oid} className={selectedOC?.oid === oc.oid ? "selected" : ""} onClick={() => setSelectedOC(oc)}>
                    <span>{oc.name}</span><small>{oc.kind}</small>
                  </button>
                ))}
              </div>
            </div>

            <div className="schema-list-block attributetype-block">
              <h3>AttributeTypes ({schema?.attributeTypes.length ?? 0})</h3>
              <input className="filter-input" value={attrFilter} onChange={(e) => setAttrFilter(e.target.value)} placeholder="Filter attributeType" />
              <div className="attribute-list">
                {filteredAttributes.map((attr) => (
                  <button
                    key={`${attr.oid}-${primaryAttrName(attr)}`}
                    className={selectedAttr?.oid === attr.oid ? "selected" : ""}
                    onClick={() => setSelectedAttr(attr)}
                  >
                    <span>{primaryAttrName(attr)}</span>
                    <small>{attr.oid}</small>
                  </button>
                ))}
              </div>
            </div>
          </div>
        </div>

        <div className="panel details-panel">
          <header>Schema details</header>
          <div className="detail-stack">
            <section className="objectclass-details">
              <h2>{selectedOC?.name ?? "objectClass не выбран"}</h2>
              {selectedOC ? (
                <>
                  <p className="muted">{selectedOC.oid} · {selectedOC.kind}</p>
                  <div className="columns">
                    <AttrList title="MUST attributes" attrs={selectedOC.must ?? []} />
                    <AttrList title="MAY attributes" attrs={selectedOC.may ?? []} />
                  </div>
                </>
              ) : <p className="muted">Загрузите схему, чтобы увидеть objectClasses.</p>}
            </section>

            <OptionalAttributeSelector
              attrs={optionalAttributeNames}
              selected={config.selectedAttributes ?? {}}
              onToggle={toggleOptionalAttribute}
              onToggleAll={setAllOptionalAttributes}
            />

            <section className="attribute-details compact-details">
              <h2>{selectedAttr ? primaryAttrName(selectedAttr) : "attributeType не выбран"}</h2>
              {selectedAttr ? (
                <>
                  <p className="muted">{selectedAttr.oid}</p>
                  <dl>
                    <dt>Aliases</dt>
                    <dd>{(selectedAttr.names ?? []).join(", ") || "-"}</dd>
                    <dt>SUP</dt>
                    <dd>{selectedAttr.sup || "-"}</dd>
                    <dt>Syntax</dt>
                    <dd>{selectedAttr.syntax || "-"}</dd>
                    <dt>Flags</dt>
                    <dd>{[selectedAttr.singleValue ? "SINGLE-VALUE" : ""].filter(Boolean).join(", ") || "-"}</dd>
                  </dl>
                </>
              ) : <p className="muted">Выберите attributeType слева.</p>}
            </section>
          </div>
        </div>

        <div className="panel config-panel">
          <header>Генерация</header>
          <div className="form-grid single">
            <Field label="Base DN"><input value={config.baseDN} onChange={(e) => updateConfig({ baseDN: e.target.value })} /></Field>
            <Field label="Output LDIF"><div className="inline"><input value={config.outputPath} onChange={(e) => updateConfig({ outputPath: e.target.value })} /><button onClick={onSelectOutputPath} title="Выбрать output file"><FolderOpen size={17} /></button></div></Field>
            <Field label="Количество записей"><input type="number" value={config.count} onChange={(e) => updateConfig({ count: Number(e.target.value) })} /></Field>
            <Field label="Заполнение MAY, %"><input type="number" value={config.optionalFillPercent} onChange={(e) => updateConfig({ optionalFillPercent: Number(e.target.value) })} /></Field>
          </div>
          <label className="check"><input type="checkbox" checked={config.strictMode} onChange={(e) => updateConfig({ strictMode: e.target.checked })} /> Строгая проверка схемы</label>
          
          <h3>ObjectClass для типа записи</h3>
          <div className="mapping compact">
            {entryTypes.map((t) => (
              <Field key={t.key} label={t.label}>
                <select value={config.objectClasses?.[t.key]?.[0] ?? ""} onChange={(e) => setObjectClass(t.key, e.target.value)}>
                  <option value="">Выберите ObjectClass</option>
                  {objectClassNames.map((name) => <option key={name}>{name}</option>)}
                </select>
              </Field>
            ))}
          </div>

          <h3>Доли типов записей</h3>
          <div className="form-grid">
            <Field label="Привилегированные, %"><input type="number" value={config.tree.privilegedPercent} onChange={(e) => updateTreePercent("privilegedPercent", Number(e.target.value))} /></Field>
            <Field label="Группы, %"><input type="number" value={config.tree.groupPercent} onChange={(e) => updateTreePercent("groupPercent", Number(e.target.value))} /></Field>
            <Field label="Компьютеры, %"><input type="number" value={config.tree.computerPercent} onChange={(e) => updateTreePercent("computerPercent", Number(e.target.value))} /></Field>
            <Field label="Сервисы, %"><input type="number" value={config.tree.servicePercent} onChange={(e) => updateTreePercent("servicePercent", Number(e.target.value))} /></Field>
          </div>

          <h3>Связи</h3>
          <div className="form-grid">
            <Field label="Пользователи в группах, %"><input type="number" value={config.relationships.usersInGroupsPercent} onChange={(e) => updateRel("usersInGroupsPercent", Number(e.target.value))} /></Field>
            <Field label="Вложенные группы, %"><input type="number" value={config.relationships.nestedGroupsPercent} onChange={(e) => updateRel("nestedGroupsPercent", Number(e.target.value))} /></Field>
            <Field label="Менеджеры, %"><input type="number" value={config.relationships.managersPercent} onChange={(e) => updateRel("managersPercent", Number(e.target.value))} /></Field>
          </div>
        </div>
      </section>
    </main>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="field"><span>{label}</span>{children}</label>;
}

function AttrList({ title, attrs }: { title: string; attrs: string[] }) {
  return (
    <div>
      <h3>{title}</h3>
      <div className="chips readonly">
        {attrs.map((attr) => <span key={attr}>{attr}</span>)}
      </div>
    </div>
  );
}

function OptionalAttributeSelector({ attrs, selected, onToggle, onToggleAll }: { attrs: string[]; selected: Record<string, boolean>; onToggle(name: string): void; onToggleAll(enabled: boolean): void }) {
  const allEnabled = attrs.length > 0 && attrs.every((attr) => isAttributeEnabled(selected, attr));
  return (
    <section className="optional-selector">
      <div className="optional-header">
        <div>
          <h3>Allowed optional attributes</h3>
          <p className="muted">Глобальный список MAY-атрибутов, которые можно добавлять для любых objectClass.</p>
        </div>
        <label>
          <input type="checkbox" checked={allEnabled} onChange={(e) => onToggleAll(e.target.checked)} />
          Select all
        </label>
      </div>
      <div className="optional-list">
        {attrs.map((attr) => (
          <label key={attr}>
            <input type="checkbox" checked={isAttributeEnabled(selected, attr)} onChange={() => onToggle(attr)} />
            {attr}
          </label>
        ))}
      </div>
    </section>
  );
}

function isAttributeEnabled(selected: Record<string, boolean>, attr: string) {
  return Object.keys(selected).length === 0 || selected[attr.toLowerCase()] === true;
}

function materializeSelectedAttributes(selected: Record<string, boolean>, attrs: string[], defaultValue = true) {
  if (Object.keys(selected).length > 0) {
    return { ...selected };
  }
  return Object.fromEntries(attrs.map((attr) => [attr.toLowerCase(), defaultValue]));
}

function formatBytes(bytes: number) {
  if (bytes > 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  if (bytes > 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${bytes} B`;
}

function primaryAttrName(attr: AttributeType) {
  return attr.names?.[0] || attr.oid || "unknown";
}

function normalizeSchema(value: Partial<SchemaSummary> | null | undefined): SchemaSummary {
  return {
    attributeTypes: Array.isArray(value?.attributeTypes) ? value.attributeTypes : [],
    objectClasses: Array.isArray(value?.objectClasses)
      ? value.objectClasses.map((oc) => ({
          ...oc,
          name: oc.name || oc.oid || "unknown",
          oid: oc.oid || oc.name || "unknown",
          kind: oc.kind || "STRUCTURAL",
          must: Array.isArray(oc.must) ? oc.must : [],
          may: Array.isArray(oc.may) ? oc.may : [],
          sup: Array.isArray(oc.sup) ? oc.sup : [],
          warnings: Array.isArray(oc.warnings) ? oc.warnings : [],
        }))
      : [],
    warnings: Array.isArray(value?.warnings) ? value.warnings : [],
  };
}

function normalizeConfig(value: Partial<GeneratorConfig> | null | undefined): GeneratorConfig {
  const fallback: GeneratorConfig = {
    baseDN: "dc=example,dc=com",
    count: 100000,
    seed: 42,
    batchSize: 1000,
    outputPath: "generated.ldif",
    strictMode: true,
    optionalFillPercent: 45,
    selectedAttributes: {},
    objectClasses: {
      user: ["inetOrgPerson"],
      privilegedUser: ["privUser"],
      group: ["groupOfNames"],
      computer: ["device"],
      serviceAccount: ["serviceUser"],
    },
    tree: {
      mode: "hierarchical",
      userOU: "Users",
      privilegedOU: "PrivilegedUsers",
      groupOU: "Groups",
      computerOU: "Computers",
      serviceOU: "ServiceAccounts",
      privilegedPercent: 3,
      groupPercent: 5,
      computerPercent: 5,
      servicePercent: 2,
    },
    relationships: {
      usersInGroupsPercent: 70,
      nestedGroupsPercent: 10,
      managersPercent: 15,
      maxMembersPerGroup: 200,
    },
  };
  const cfg = { ...fallback, ...(value ?? {}) };
  return {
    ...cfg,
    count: Number(cfg.count) || fallback.count,
    seed: Number(cfg.seed) || fallback.seed,
    batchSize: Number(cfg.batchSize) || fallback.batchSize,
    optionalFillPercent: Number(cfg.optionalFillPercent) || 0,
    selectedAttributes: cfg.selectedAttributes ?? {},
    objectClasses: { ...fallback.objectClasses, ...(cfg.objectClasses ?? {}) },
    tree: { ...fallback.tree, ...(cfg.tree ?? {}) },
    relationships: { ...fallback.relationships, ...(cfg.relationships ?? {}) },
  };
}

function errorText(err: unknown) {
  if (err instanceof Error) return err.message;
  if (typeof err === "string") return err;
  try {
    return JSON.stringify(err);
  } catch {
    return "Unknown error";
  }
}
