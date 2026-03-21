export namespace db {
	
	export class ConfigMapping {
	    configPath: string;
	    modID: string;
	    confidence: number;
	    isManual: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConfigMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configPath = source["configPath"];
	        this.modID = source["modID"];
	        this.confidence = source["confidence"];
	        this.isManual = source["isManual"];
	    }
	}
	export class Mod {
	    id: string;
	    name: string;
	    version: string;
	    description: string;
	    authors: string;
	    modLoader: string;
	    jarFileName: string;
	    jarSHA1: string;
	    jarSHA512: string;
	    fingerprint: number;
	    homepageURL: string;
	    curseForgeID: number;
	    modrinthID: string;
	    curseForgeURL: string;
	    modrinthURL: string;
	    iconURL: string;
	    providedIDs: string;
	    isLibrary: boolean;
	    // Go type: time
	    lastScanned: any;
	    // Go type: time
	    lastAPICheck: any;
	    onlineDesc: string;
	    loaders: string;
	    categories: string;
	    projectType: string;
	
	    static createFrom(source: any = {}) {
	        return new Mod(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.authors = source["authors"];
	        this.modLoader = source["modLoader"];
	        this.jarFileName = source["jarFileName"];
	        this.jarSHA1 = source["jarSHA1"];
	        this.jarSHA512 = source["jarSHA512"];
	        this.fingerprint = source["fingerprint"];
	        this.homepageURL = source["homepageURL"];
	        this.curseForgeID = source["curseForgeID"];
	        this.modrinthID = source["modrinthID"];
	        this.curseForgeURL = source["curseForgeURL"];
	        this.modrinthURL = source["modrinthURL"];
	        this.iconURL = source["iconURL"];
	        this.providedIDs = source["providedIDs"];
	        this.isLibrary = source["isLibrary"];
	        this.lastScanned = this.convertValues(source["lastScanned"], null);
	        this.lastAPICheck = this.convertValues(source["lastAPICheck"], null);
	        this.onlineDesc = source["onlineDesc"];
	        this.loaders = source["loaders"];
	        this.categories = source["categories"];
	        this.projectType = source["projectType"];
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
	export class Settings {
	    instancePath: string;
	    curseForgeAPIKey: string;
	    modrinthAPIKey: string;
	    cacheTTLHours: number;
	    appScale: number;
	    customModrinthRoot: string;
	    customCurseForgeRoot: string;
	    customFTBRoot: string;
	    customLauncherRoots: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instancePath = source["instancePath"];
	        this.curseForgeAPIKey = source["curseForgeAPIKey"];
	        this.modrinthAPIKey = source["modrinthAPIKey"];
	        this.cacheTTLHours = source["cacheTTLHours"];
	        this.appScale = source["appScale"];
	        this.customModrinthRoot = source["customModrinthRoot"];
	        this.customCurseForgeRoot = source["customCurseForgeRoot"];
	        this.customFTBRoot = source["customFTBRoot"];
	        this.customLauncherRoots = source["customLauncherRoots"];
	    }
	}

}

export namespace embeddings {
	
	export class SearchResult {
	    mod: db.Mod;
	    score: number;
	    matchType: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mod = this.convertValues(source["mod"], db.Mod);
	        this.score = source["score"];
	        this.matchType = source["matchType"];
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

export namespace instance {
	
	export class Instance {
	    name: string;
	    path: string;
	    launcher: string;
	    hasMods: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Instance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.launcher = source["launcher"];
	        this.hasMods = source["hasMods"];
	    }
	}

}

export namespace main {
	
	export class DetailDependency {
	    modID: string;
	    depModID: string;
	    depName: string;
	    type: string;
	    satisfied: boolean;
	    source: string;
	    resolvedModID?: string;
	    resolvedName?: string;
	    resolvedIconURL?: string;
	    sources?: string[];
	
	    static createFrom(source: any = {}) {
	        return new DetailDependency(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modID = source["modID"];
	        this.depModID = source["depModID"];
	        this.depName = source["depName"];
	        this.type = source["type"];
	        this.satisfied = source["satisfied"];
	        this.source = source["source"];
	        this.resolvedModID = source["resolvedModID"];
	        this.resolvedName = source["resolvedName"];
	        this.resolvedIconURL = source["resolvedIconURL"];
	        this.sources = source["sources"];
	    }
	}
	export class IncomingMixin {
	    ownerModID: string;
	    ownerModName: string;
	    ownerIconURL?: string;
	    mixinClass: string;
	    targetClass: string;
	    targetMembers: string;
	
	    static createFrom(source: any = {}) {
	        return new IncomingMixin(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ownerModID = source["ownerModID"];
	        this.ownerModName = source["ownerModName"];
	        this.ownerIconURL = source["ownerIconURL"];
	        this.mixinClass = source["mixinClass"];
	        this.targetClass = source["targetClass"];
	        this.targetMembers = source["targetMembers"];
	    }
	}
	export class MixinDetail {
	    mixinClass: string;
	    targetClass: string;
	    targetModID: string;
	    targetModName?: string;
	    targetMembers: string;
	
	    static createFrom(source: any = {}) {
	        return new MixinDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mixinClass = source["mixinClass"];
	        this.targetClass = source["targetClass"];
	        this.targetModID = source["targetModID"];
	        this.targetModName = source["targetModName"];
	        this.targetMembers = source["targetMembers"];
	    }
	}
	export class UnresolvedExternalDepLink {
	    depModID: string;
	    depName?: string;
	    type: string;
	    source: string;
	    openURL?: string;
	
	    static createFrom(source: any = {}) {
	        return new UnresolvedExternalDepLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.depModID = source["depModID"];
	        this.depName = source["depName"];
	        this.type = source["type"];
	        this.source = source["source"];
	        this.openURL = source["openURL"];
	    }
	}
	export class ModDetail {
	    mod: db.Mod;
	    dependencies: DetailDependency[];
	    configs: db.ConfigMapping[];
	    providedModules: string[];
	    unresolvedExternal?: UnresolvedExternalDepLink[];
	    mixins?: MixinDetail[];
	    incomingMixins?: IncomingMixin[];
	
	    static createFrom(source: any = {}) {
	        return new ModDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mod = this.convertValues(source["mod"], db.Mod);
	        this.dependencies = this.convertValues(source["dependencies"], DetailDependency);
	        this.configs = this.convertValues(source["configs"], db.ConfigMapping);
	        this.providedModules = source["providedModules"];
	        this.unresolvedExternal = this.convertValues(source["unresolvedExternal"], UnresolvedExternalDepLink);
	        this.mixins = this.convertValues(source["mixins"], MixinDetail);
	        this.incomingMixins = this.convertValues(source["incomingMixins"], IncomingMixin);
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
	export class ReverseDep {
	    modID: string;
	    name: string;
	    iconURL: string;
	    type: string;
	    via?: string;
	
	    static createFrom(source: any = {}) {
	        return new ReverseDep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modID = source["modID"];
	        this.name = source["name"];
	        this.iconURL = source["iconURL"];
	        this.type = source["type"];
	        this.via = source["via"];
	    }
	}

}

export namespace resolver {
	
	export class GraphLink {
	    source: string;
	    target: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new GraphLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.target = source["target"];
	        this.type = source["type"];
	    }
	}
	export class GraphNode {
	    id: string;
	    name: string;
	    modLoader: string;
	    loaders: string;
	    isLibrary: boolean;
	    group: string;
	    iconURL: string;
	
	    static createFrom(source: any = {}) {
	        return new GraphNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.modLoader = source["modLoader"];
	        this.loaders = source["loaders"];
	        this.isLibrary = source["isLibrary"];
	        this.group = source["group"];
	        this.iconURL = source["iconURL"];
	    }
	}
	export class Graph {
	    nodes: GraphNode[];
	    links: GraphLink[];
	
	    static createFrom(source: any = {}) {
	        return new Graph(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], GraphNode);
	        this.links = this.convertValues(source["links"], GraphLink);
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
	
	
	export class MissingDep {
	    requiredBy: string;
	    depModID: string;
	    depName: string;
	
	    static createFrom(source: any = {}) {
	        return new MissingDep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requiredBy = source["requiredBy"];
	        this.depModID = source["depModID"];
	        this.depName = source["depName"];
	    }
	}

}

export namespace scanner {
	
	export class ConfigFile {
	    path: string;
	    absolutePath: string;
	    fileName: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.absolutePath = source["absolutePath"];
	        this.fileName = source["fileName"];
	    }
	}

}

