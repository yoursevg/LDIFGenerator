import type { GeneratorConfig, Progress, Report, SchemaSummary } from "./types";

type Backend = {
  app?: {
    Service?: {
      LoadSchema(paths: string[]): Promise<SchemaSummary>;
      SelectSchemaFiles(): Promise<string[]>;
      SelectOutputPath(): Promise<string>;
      Generate(config: GeneratorConfig): Promise<Report>;
      CancelGeneration(): Promise<void>;
      Progress(): Promise<Progress>;
      DefaultConfig(): Promise<GeneratorConfig>;
    };
  };
};

const backend = () => (window as Window & { go?: Backend }).go?.app?.Service;

export async function defaultConfig(): Promise<GeneratorConfig> {
  const svc = backend();
  if (svc) return svc.DefaultConfig();
  return {
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
}

export async function loadSchema(paths: string[]) {
  const svc = backend();
  if (!svc) throw new Error("Wails backend is not available. Run with wails dev/build.");
  return svc.LoadSchema(paths);
}

export async function selectSchemaFiles() {
  const svc = backend();
  if (!svc) throw new Error("Wails backend is not available. Run with wails dev/build.");
  return svc.SelectSchemaFiles();
}

export async function selectOutputPath() {
  const svc = backend();
  if (!svc) throw new Error("Wails backend is not available. Run with wails dev/build.");
  return svc.SelectOutputPath();
}

export async function generate(config: GeneratorConfig) {
  const svc = backend();
  if (!svc) throw new Error("Wails backend is not available. Run with wails dev/build.");
  return svc.Generate(config);
}

export async function cancelGeneration() {
  await backend()?.CancelGeneration();
}

export async function progress() {
  return backend()?.Progress() ?? { written: 0, total: 0 };
}
