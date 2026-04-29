export type EntryType = "user" | "privilegedUser" | "group" | "computer" | "serviceAccount";
export type TreeMode = "flat" | "ou" | "hierarchical";

export type AttributeType = {
  oid: string;
  names: string[];
  description?: string;
  sup?: string;
  syntax?: string;
  singleValue?: boolean;
};

export type ObjectClassSummary = {
  name: string;
  oid: string;
  kind: string;
  sup?: string[];
  must: string[];
  may: string[];
  warnings?: string[];
};

export type SchemaSummary = {
  attributeTypes: AttributeType[];
  objectClasses: ObjectClassSummary[];
  warnings?: string[];
};

export type GeneratorConfig = {
  baseDN: string;
  count: number;
  seed: number;
  batchSize: number;
  outputPath: string;
  strictMode: boolean;
  optionalFillPercent: number;
  selectedAttributes: Record<string, boolean>;
  objectClasses: Record<EntryType, string[]>;
  tree: {
    mode: TreeMode;
    userOU: string;
    privilegedOU: string;
    groupOU: string;
    computerOU: string;
    serviceOU: string;
    privilegedPercent: number;
    groupPercent: number;
    computerPercent: number;
    servicePercent: number;
  };
  relationships: {
    usersInGroupsPercent: number;
    nestedGroupsPercent: number;
    managersPercent: number;
    maxMembersPerGroup: number;
  };
};

export type Progress = {
  written: number;
  total: number;
};

export type Report = {
  records: number;
  duration: number;
  recordsPerSec: number;
  fileBytes: number;
  outputPath: string;
  warnings?: string[];
};
