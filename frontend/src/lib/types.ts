// Re-export Wails-generated types and add frontend-only types

export type { db, main, resolver, embeddings, instance, scanner } from '../../wailsjs/go/models';

// Convenience type aliases using generated models
import type { db, main, resolver, embeddings, instance, scanner } from '../../wailsjs/go/models';

export interface Mod extends db.Mod {
}
export type Dependency = main.DetailDependency;
export interface DetailDependency extends main.DetailDependency {
	resolvedModID?: string;
	resolvedName?: string;
	resolvedIconURL?: string;
	sources?: string[];
}
export interface UnresolvedExternalDependency {
	depModID: string;
	depName?: string;
	type: string;
	source: string;
	openURL?: string;
}
export type ConfigMapping = db.ConfigMapping;
export type Settings = db.Settings;
export type LibraryDetectionDebug = main.LibraryDetectionDebug;
export interface InstanceTextFile {
	name: string;
	relativePath: string;
	size: number;
	modifiedUnix: number;
}
export interface LogsOverview {
	available: boolean;
	defaultLiveLog: string;
	latestCrash?: InstanceTextFile;
	crashReports: InstanceTextFile[];
	logFiles: InstanceTextFile[];
}
export interface TextFileContent {
	name: string;
	relativePath: string;
	content: string;
	totalSize: number;
	modifiedUnix: number;
	truncated: boolean;
	missing: boolean;
}
export interface LiveLogChunk {
	relativePath: string;
	content: string;
	totalSize: number;
	modifiedUnix: number;
}
export interface ModDetail {
	mod: Mod;
	dependencies: DetailDependency[];
	configs: ConfigMapping[];
	providedModules?: string[];
	libraryDetection: LibraryDetectionDebug;
	unresolvedExternal?: UnresolvedExternalDependency[];
	mixins?: MixinDetail[];
	incomingMixins?: IncomingMixin[];
}
export interface MixinDetail {
	mixinClass: string;
	targetClass: string;
	targetModID: string;
	targetModName?: string;
	targetMembers: string;
}
export interface IncomingMixin {
	ownerModID: string;
	ownerModName: string;
	ownerIconURL?: string;
	mixinClass: string;
	targetClass: string;
	targetMembers: string;
}
export type SearchResult = embeddings.SearchResult;
export type DependencyGraph = resolver.Graph;
export type GraphNode = resolver.GraphNode;
export type GraphLink = resolver.GraphLink;
export type MissingDep = resolver.MissingDep;
export type Instance = instance.Instance;
export type ConfigFile = scanner.ConfigFile;
export interface ReverseDep extends main.ReverseDep {
	via?: string;
}

export type View = 'mods' | 'graph' | 'logs' | 'settings';
