export namespace app {
	
	export class ObjectClassSummary {
	    name: string;
	    oid: string;
	    kind: string;
	    sup?: string[];
	    must: string[];
	    may: string[];
	    warnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ObjectClassSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.oid = source["oid"];
	        this.kind = source["kind"];
	        this.sup = source["sup"];
	        this.must = source["must"];
	        this.may = source["may"];
	        this.warnings = source["warnings"];
	    }
	}
	export class Progress {
	    written: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new Progress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.written = source["written"];
	        this.total = source["total"];
	    }
	}
	export class SchemaSummary {
	    attributeTypes: schema.AttributeType[];
	    objectClasses: ObjectClassSummary[];
	    warnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new SchemaSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attributeTypes = this.convertValues(source["attributeTypes"], schema.AttributeType);
	        this.objectClasses = this.convertValues(source["objectClasses"], ObjectClassSummary);
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace generator {
	
	export class RelationshipConfig {
	    usersInGroupsPercent: number;
	    nestedGroupsPercent: number;
	    managersPercent: number;
	    maxMembersPerGroup: number;
	
	    static createFrom(source: any = {}) {
	        return new RelationshipConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.usersInGroupsPercent = source["usersInGroupsPercent"];
	        this.nestedGroupsPercent = source["nestedGroupsPercent"];
	        this.managersPercent = source["managersPercent"];
	        this.maxMembersPerGroup = source["maxMembersPerGroup"];
	    }
	}
	export class TreeConfig {
	    mode: string;
	    userOU: string;
	    privilegedOU: string;
	    groupOU: string;
	    computerOU: string;
	    serviceOU: string;
	    privilegedPercent: number;
	    groupPercent: number;
	    computerPercent: number;
	    servicePercent: number;
	
	    static createFrom(source: any = {}) {
	        return new TreeConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.userOU = source["userOU"];
	        this.privilegedOU = source["privilegedOU"];
	        this.groupOU = source["groupOU"];
	        this.computerOU = source["computerOU"];
	        this.serviceOU = source["serviceOU"];
	        this.privilegedPercent = source["privilegedPercent"];
	        this.groupPercent = source["groupPercent"];
	        this.computerPercent = source["computerPercent"];
	        this.servicePercent = source["servicePercent"];
	    }
	}
	export class GeneratorConfig {
	    baseDN: string;
	    count: number;
	    seed: number;
	    batchSize: number;
	    outputPath: string;
	    strictMode: boolean;
	    optionalFillPercent: number;
	    selectedAttributes: Record<string, boolean>;
	    objectClasses: Record<string, Array<string>>;
	    tree: TreeConfig;
	    relationships: RelationshipConfig;
	
	    static createFrom(source: any = {}) {
	        return new GeneratorConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseDN = source["baseDN"];
	        this.count = source["count"];
	        this.seed = source["seed"];
	        this.batchSize = source["batchSize"];
	        this.outputPath = source["outputPath"];
	        this.strictMode = source["strictMode"];
	        this.optionalFillPercent = source["optionalFillPercent"];
	        this.selectedAttributes = source["selectedAttributes"];
	        this.objectClasses = source["objectClasses"];
	        this.tree = this.convertValues(source["tree"], TreeConfig);
	        this.relationships = this.convertValues(source["relationships"], RelationshipConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Report {
	    records: number;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt: any;
	    duration: number;
	    recordsPerSec: number;
	    fileBytes: number;
	    outputPath: string;
	    warnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.records = source["records"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
	        this.duration = source["duration"];
	        this.recordsPerSec = source["recordsPerSec"];
	        this.fileBytes = source["fileBytes"];
	        this.outputPath = source["outputPath"];
	        this.warnings = source["warnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace schema {
	
	export class AttributeType {
	    oid: string;
	    names: string[];
	    description?: string;
	    sup?: string;
	    equality?: string;
	    ordering?: string;
	    substr?: string;
	    syntax?: string;
	    singleValue?: boolean;
	    noUserMod?: boolean;
	    usage?: string;
	
	    static createFrom(source: any = {}) {
	        return new AttributeType(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.oid = source["oid"];
	        this.names = source["names"];
	        this.description = source["description"];
	        this.sup = source["sup"];
	        this.equality = source["equality"];
	        this.ordering = source["ordering"];
	        this.substr = source["substr"];
	        this.syntax = source["syntax"];
	        this.singleValue = source["singleValue"];
	        this.noUserMod = source["noUserMod"];
	        this.usage = source["usage"];
	    }
	}

}

