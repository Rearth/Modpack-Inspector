import { useEffect, useRef, useState, useCallback, useMemo, useLayoutEffect } from 'react';
import { GetDependencyGraph } from '../../wailsjs/go/main/App';
import { forceX, forceY, forceCollide } from 'd3-force';
import ForceGraph2D from 'react-force-graph-2d';
import type { DependencyGraph, GraphNode, GraphLink } from '../lib/types';

interface ModGraphProps {
  onSelectMod: (id: string) => void;
}

const LOADER_COLORS: Record<string, string> = {
  fabric: '#6366f1',
  forge: '#ea580c',
  neoforge: '#ea580c',
  quilt: '#a855f7',
};

const GROUP_COLORS: Record<string, string> = {
  normal: '#3b82f6',
  library: '#9ca3af',
  unused: '#f59e0b',
  missing: '#ef4444',
};

const GROUP_POSITIONS: Record<string, { x: number; y: number }> = {
  normal:  { x: 0.3, y: 0.3 },
  library: { x: 0.7, y: 0.7 },
  unused:  { x: 0.85, y: 0.2 },
  missing: { x: 0.15, y: 0.85 },
};

const HOVER_CLEAR_DELAY_MS = 120;
const HOVER_TRANSITION_MS = 180;
const CONNECTED_LABEL_ZOOM_THRESHOLD = 0.5;
const DEFAULT_LABEL_ZOOM_THRESHOLD = 0.95;

interface HoverTransition {
  from: string | null;
  to: string | null;
  progress: number;
}

function getLinkEndpointID(endpoint: string | GraphNode): string {
  return typeof endpoint === 'object' ? endpoint.id : endpoint;
}

function getUndirectedLinkKey(source: string, target: string): string {
  return source < target ? `${source}__${target}` : `${target}__${source}`;
}

function getNodeColor(node: GraphNode): string {
  if (node.group === 'missing') return GROUP_COLORS.missing;
  if (node.group === 'unused') return GROUP_COLORS.unused;
  if (node.group === 'library') return GROUP_COLORS.library;
  return LOADER_COLORS[node.modLoader] || GROUP_COLORS.normal;
}

function getNodeRadius(node: GraphNode, connectionCount: Record<string, number>): number {
  const connections = connectionCount[node.id] || 0;
  return Math.min(14 + connections * 0.9, 26);
}

function getFallbackDimensions() {
  if (typeof window === 'undefined') {
    return { width: 0, height: 0 };
  }
  return {
    width: Math.max(window.innerWidth - 60, 0),
    height: Math.max(window.innerHeight, 0),
  };
}

function mix(from: number, to: number, progress: number): number {
  return from + (to - from) * progress;
}

function getNodeFocus(nodeID: string, hoveredID: string | null, connected: Set<string>): number {
  if (!hoveredID) return 0;
  if (nodeID === hoveredID) return 1;
  if (connected.has(`${hoveredID}__${nodeID}`) || connected.has(`${nodeID}__${hoveredID}`)) return 0.45;
  return -0.55;
}

function getLinkFocus(src: string, tgt: string, hoveredID: string | null, connected: Set<string>): number {
  if (!hoveredID) return 0;
  if (connected.has(`${src}__${tgt}`)) return 1;
  return -0.65;
}

function isFabricAPINode(node: GraphNode): boolean {
  return node.id.toLowerCase() === 'fabric-api' || (node.name || '').toLowerCase() === 'fabric api';
}

export function ModGraph({ onSelectMod }: ModGraphProps) {
  const [graph, setGraph] = useState<DependencyGraph | null>(null);
  const [loading, setLoading] = useState(true);
  const [dimensions, setDimensions] = useState(getFallbackDimensions);
  const [hoverTransition, setHoverTransition] = useState<HoverTransition>({ from: null, to: null, progress: 1 });
  const [isSettingsPanelOpen, setIsSettingsPanelOpen] = useState(true);
  const [showZoomedLabels, setShowZoomedLabels] = useState(false);
  const [hideOptionalMissingNodes, setHideOptionalMissingNodes] = useState(true);
  const [hideUnconnectedNodes, setHideUnconnectedNodes] = useState(false);
  const [showOptionalConnections, setShowOptionalConnections] = useState(false);
  const [showFabricAPI, setShowFabricAPI] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const graphRef = useRef<any>(null);
  const imageCache = useRef<Record<string, HTMLImageElement>>({});
  const forcesApplied = useRef(false);
  const hoverClearTimeout = useRef<number | null>(null);
  const hoverAnimationFrame = useRef<number | null>(null);
  const hoverTransitionRef = useRef<HoverTransition>({ from: null, to: null, progress: 1 });

  // Track container size with ResizeObserver
  const measureContainer = useCallback(() => {
    const el = containerRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const width = Math.floor(rect.width);
    const height = Math.floor(rect.height);
    if (width > 0 && height > 0) {
      setDimensions(prev => (prev.width === width && prev.height === height ? prev : { width, height }));
    }
  }, []);

  useLayoutEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver(entries => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        if (width > 0 && height > 0) {
          setDimensions({ width: Math.floor(width), height: Math.floor(height) });
        }
      }
    });
    ro.observe(el);
    const raf = requestAnimationFrame(measureContainer);
    const interval = window.setInterval(measureContainer, 250);
    window.addEventListener('resize', measureContainer);
    measureContainer();
    return () => {
      cancelAnimationFrame(raf);
      window.clearInterval(interval);
      window.removeEventListener('resize', measureContainer);
      ro.disconnect();
    };
  }, [measureContainer]);

  useEffect(() => {
    return () => {
      if (hoverClearTimeout.current !== null) {
        window.clearTimeout(hoverClearTimeout.current);
      }
      if (hoverAnimationFrame.current !== null) {
        cancelAnimationFrame(hoverAnimationFrame.current);
      }
    };
  }, []);

  // Load graph data
  useEffect(() => {
    setLoading(true);
    GetDependencyGraph()
      .then(g => setGraph(g))
      .catch(console.error)
      .finally(() => setLoading(false));
  }, []);

  // Preload mod icons
  useEffect(() => {
    if (!graph) return;
    graph.nodes.forEach((node: any) => {
      if (node.iconURL && !imageCache.current[node.id]) {
        const img = new Image();
        img.src = node.iconURL;
        img.onload = () => { imageCache.current[node.id] = img; };
      }
    });
  }, [graph]);

  // Connection counts for node sizing
  const optionalMissingNodeIDs = useMemo(() => {
    if (!graph) return new Set<string>();

    const linkKinds = new Map<string, { hasOptional: boolean; hasNonOptional: boolean }>();
    for (const link of graph.links) {
      const isOptional = link.type === 'optional';
      for (const nodeID of [getLinkEndpointID(link.source), getLinkEndpointID(link.target)]) {
        const current = linkKinds.get(nodeID) || { hasOptional: false, hasNonOptional: false };
        if (isOptional) {
          current.hasOptional = true;
        } else {
          current.hasNonOptional = true;
        }
        linkKinds.set(nodeID, current);
      }
    }

    const hidden = new Set<string>();
    for (const node of graph.nodes) {
      if (node.group !== 'missing') continue;
      const kinds = linkKinds.get(node.id);
      if (kinds?.hasOptional && !kinds.hasNonOptional) {
        hidden.add(node.id);
      }
    }
    return hidden;
  }, [graph]);

  const fabricAPINodeIDs = useMemo(() => {
    if (!graph) return new Set<string>();
    return new Set(graph.nodes.filter(isFabricAPINode).map(node => node.id));
  }, [graph]);

  const hasFabricAPI = fabricAPINodeIDs.size > 0;

  const filteredGraph = useMemo(() => {
    if (!graph) return null;

    const links = graph.links.flatMap(link => {
      const source = getLinkEndpointID(link.source);
      const target = getLinkEndpointID(link.target);

      if (!showFabricAPI && (fabricAPINodeIDs.has(source) || fabricAPINodeIDs.has(target))) {
        return [];
      }

      if (!showOptionalConnections && link.type === 'optional') {
        return [];
      }
      if (hideOptionalMissingNodes && optionalMissingNodeIDs.has(target)) {
        return [];
      }
      if (hideOptionalMissingNodes && optionalMissingNodeIDs.has(source)) {
        return [];
      }
      return [{ source, target, type: link.type }];
    });

    const baseNodes = graph.nodes.filter(node => {
      if (!showFabricAPI && fabricAPINodeIDs.has(node.id)) {
        return false;
      }
      if (hideOptionalMissingNodes && optionalMissingNodeIDs.has(node.id)) {
        return false;
      }
      return true;
    });

    if (!hideUnconnectedNodes) {
      return { nodes: baseNodes, links } as DependencyGraph;
    }

    const connectedNodeIDs = new Set<string>();
    for (const link of links) {
      connectedNodeIDs.add(link.source);
      connectedNodeIDs.add(link.target);
    }

    const nodes = baseNodes.filter(node => connectedNodeIDs.has(node.id));

    return { nodes, links } as DependencyGraph;
  }, [fabricAPINodeIDs, graph, hideOptionalMissingNodes, hideUnconnectedNodes, optionalMissingNodeIDs, showFabricAPI, showOptionalConnections]);

  const connectionCount = useMemo(() => {
    if (!filteredGraph) return {};
    const counts: Record<string, number> = {};
    for (const link of filteredGraph.links) {
      const src = typeof link.source === 'object' ? (link.source as any).id : link.source;
      const tgt = typeof link.target === 'object' ? (link.target as any).id : link.target;
      counts[src] = (counts[src] || 0) + 1;
      counts[tgt] = (counts[tgt] || 0) + 1;
    }
    return counts;
  }, [filteredGraph]);

  const linkCurvature = useMemo(() => {
    if (!filteredGraph) return new Map<string, number>();

    const adjacency = new Map<string, Set<string>>();
    for (const link of filteredGraph.links) {
      const source = getLinkEndpointID(link.source);
      const target = getLinkEndpointID(link.target);

      if (!adjacency.has(source)) adjacency.set(source, new Set<string>());
      if (!adjacency.has(target)) adjacency.set(target, new Set<string>());
      adjacency.get(source)?.add(target);
      adjacency.get(target)?.add(source);
    }

    const curvatureByLink = new Map<string, number>();
    for (const link of filteredGraph.links) {
      const source = getLinkEndpointID(link.source);
      const target = getLinkEndpointID(link.target);
      const sourceNeighbors = adjacency.get(source) || new Set<string>();
      const targetNeighbors = adjacency.get(target) || new Set<string>();

      let sharedNeighbors = 0;
      for (const neighbor of sourceNeighbors) {
        if (neighbor !== target && targetNeighbors.has(neighbor)) {
          sharedNeighbors += 1;
        }
      }

      const sourceDegree = sourceNeighbors.size;
      const targetDegree = targetNeighbors.size;
      const congested = sharedNeighbors > 0 || (sourceDegree > 1 && targetDegree > 1);

      if (!congested) {
        curvatureByLink.set(getUndirectedLinkKey(source, target), 0);
        continue;
      }

      const baseCurvature = link.type === 'required' ? 0.07 : 0.11;
      const extraCurvature = Math.min(sharedNeighbors, 2) * 0.025;
      const direction = getUndirectedLinkKey(source, target).charCodeAt(0) % 2 === 0 ? 1 : -1;
      curvatureByLink.set(
        getUndirectedLinkKey(source, target),
        direction * (baseCurvature + extraCurvature),
      );
    }

    return curvatureByLink;
  }, [filteredGraph]);

  // Configure forces + zoom-to-fit
  useEffect(() => {
    if (!graphRef.current || !filteredGraph || dimensions.width < 200 || dimensions.height < 200) return;
    const fg = graphRef.current;

    fg.d3Force('charge')?.strength(-300);
    fg.d3Force('link')?.distance(80);
    fg.d3Force('center', null);

    // Group-based positioning forces
    fg.d3Force('groupX', forceX((node: any) => {
      const pos = GROUP_POSITIONS[node.group] || GROUP_POSITIONS.normal;
      return (pos.x - 0.5) * dimensions.width * 0.7;
    }).strength(0.12));

    fg.d3Force('groupY', forceY((node: any) => {
      const pos = GROUP_POSITIONS[node.group] || GROUP_POSITIONS.normal;
      return (pos.y - 0.5) * dimensions.height * 0.7;
    }).strength(0.12));

    fg.d3Force('collide', forceCollide((node: any) => {
      return getNodeRadius(node, connectionCount) + 5;
    }));

    if (!forcesApplied.current) {
      forcesApplied.current = true;
      fg.d3ReheatSimulation();
    }

    // Zoom-to-fit after simulation settles
    setTimeout(() => {
      fg.zoomToFit(400, 60);
    }, 2000);
  }, [filteredGraph, dimensions, connectionCount]);

  // Build set of connected link keys for hover highlighting
  const getConnectedLinks = useCallback((hoveredID: string | null) => {
    if (!hoveredID || !filteredGraph) return new Set<string>();
    const linked = new Set<string>();
    for (const link of filteredGraph.links) {
      const src = typeof link.source === 'object' ? (link.source as any).id : link.source;
      const tgt = typeof link.target === 'object' ? (link.target as any).id : link.target;
      if (src === hoveredID || tgt === hoveredID) {
        linked.add(`${src}__${tgt}`);
      }
    }
    return linked;
  }, [filteredGraph]);

  const fromHoveredLinks = useMemo(() => getConnectedLinks(hoverTransition.from), [getConnectedLinks, hoverTransition.from]);
  const toHoveredLinks = useMemo(() => getConnectedLinks(hoverTransition.to), [getConnectedLinks, hoverTransition.to]);

  // Custom node rendering
  const paintNode = useCallback((node: any, ctx: CanvasRenderingContext2D, globalScale: number) => {
    const n = node as GraphNode;
    const R = getNodeRadius(n, connectionCount);
    const { x, y } = node;
    const fromFocus = getNodeFocus(n.id, hoverTransition.from, fromHoveredLinks);
    const toFocus = getNodeFocus(n.id, hoverTransition.to, toHoveredLinks);
    const focus = mix(fromFocus, toFocus, hoverTransition.progress);
    const activeGlow = mix(
      hoverTransition.from === n.id ? 1 : 0,
      hoverTransition.to === n.id ? 1 : 0,
      hoverTransition.progress,
    );
    const alpha = focus < 0 ? 1 + focus * 0.7 : 1;

    // Hover glow
    if (activeGlow > 0.05) {
      ctx.beginPath();
      ctx.arc(x, y, R + 5, 0, 2 * Math.PI);
      ctx.fillStyle = `rgba(59, 130, 246, ${0.18 * activeGlow})`;
      ctx.fill();
    }

    // Background circle
    ctx.beginPath();
    ctx.arc(x, y, R, 0, 2 * Math.PI);
    ctx.fillStyle = getNodeColor(n);
    ctx.globalAlpha = alpha;
    ctx.fill();
    ctx.strokeStyle = activeGlow > 0.15 ? 'rgba(0,0,0,0.4)' : 'rgba(0,0,0,0.1)';
    ctx.lineWidth = activeGlow > 0.15 ? 2 : 0.8;
    ctx.stroke();

    // Icon or first letter
    const img = imageCache.current[n.id];
    if (img && img.complete && img.naturalWidth > 0) {
      const iconInset = Math.max(0.75, R * 0.08);
      const iconRadius = R - iconInset;
      ctx.save();
      ctx.beginPath();
      ctx.arc(x, y, iconRadius, 0, 2 * Math.PI);
      ctx.clip();
      ctx.drawImage(img, x - iconRadius, y - iconRadius, iconRadius * 2, iconRadius * 2);
      ctx.restore();
    } else {
      const letter = (n.name || n.id).charAt(0).toUpperCase();
      const fontSize = Math.max(R * 1.0, 8);
      ctx.font = `bold ${fontSize}px -apple-system, system-ui, sans-serif`;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillStyle = 'rgba(255,255,255,0.9)';
      ctx.fillText(letter, x, y);
    }

    ctx.globalAlpha = 1.0;

    const labelThreshold = showZoomedLabels ? DEFAULT_LABEL_ZOOM_THRESHOLD : CONNECTED_LABEL_ZOOM_THRESHOLD;
    const shouldShowLabel = globalScale > labelThreshold && (
      showZoomedLabels ? alpha > 0.55 : focus >= 0.15
    );

    if (shouldShowLabel) {
      const label = n.name || n.id;
      const displayLabel = label.length > 24 ? label.substring(0, 22) + '…' : label;
      const fontSize = Math.max(12 / globalScale, 3.5);
      ctx.font = `500 ${fontSize}px -apple-system, system-ui, sans-serif`;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'top';
      ctx.fillStyle = activeGlow > 0.15 ? 'rgba(0,0,0,0.9)' : `rgba(0,0,0,${0.25 + Math.max(alpha, 0.35) * 0.35})`;
      ctx.fillText(displayLabel, x, y + R + 3 / globalScale);
    }
  }, [connectionCount, hoverTransition, fromHoveredLinks, showZoomedLabels, toHoveredLinks]);

  // Pointer area
  const nodePointerAreaPaint = useCallback((node: any, color: string, ctx: CanvasRenderingContext2D) => {
    const R = getNodeRadius(node, connectionCount);
    ctx.beginPath();
    ctx.arc(node.x, node.y, R + 5, 0, 2 * Math.PI);
    ctx.fillStyle = color;
    ctx.fill();
  }, [connectionCount]);

  // Link color with hover highlighting
  const getLinkColor = useCallback((link: any) => {
    const src = typeof link.source === 'object' ? link.source.id : link.source;
    const tgt = typeof link.target === 'object' ? link.target.id : link.target;
    const fromFocus = getLinkFocus(src, tgt, hoverTransition.from, fromHoveredLinks);
    const toFocus = getLinkFocus(src, tgt, hoverTransition.to, toHoveredLinks);
    const focus = mix(fromFocus, toFocus, hoverTransition.progress);

    if (focus > 0.05) {
      return link.type === 'required'
        ? `rgba(59, 130, 246, ${0.2 + focus * 0.7})`
        : `rgba(59, 130, 246, ${0.15 + focus * 0.45})`;
    }
    if (focus < 0) {
      return `rgba(0, 0, 0, ${0.18 - Math.abs(focus) * 0.12})`;
    }
    return link.type === 'required' ? 'rgba(0, 0, 0, 0.2)' : 'rgba(0, 0, 0, 0.08)';
  }, [hoverTransition, fromHoveredLinks, toHoveredLinks]);

  const startHoverTransition = useCallback((nextID: string | null) => {
    if (hoverAnimationFrame.current !== null) {
      cancelAnimationFrame(hoverAnimationFrame.current);
      hoverAnimationFrame.current = null;
    }

    const current = hoverTransitionRef.current;
    if (current.to === nextID && current.progress === 1) {
      return;
    }

    const baseState: HoverTransition = {
      from: current.to,
      to: nextID,
      progress: 0,
    };
    hoverTransitionRef.current = baseState;
    setHoverTransition(baseState);

    let startTime: number | null = null;
    const tick = (timestamp: number) => {
      if (startTime === null) {
        startTime = timestamp;
      }
      const progress = Math.min((timestamp - startTime) / HOVER_TRANSITION_MS, 1);
      const nextState: HoverTransition = { ...baseState, progress };
      hoverTransitionRef.current = nextState;
      setHoverTransition(nextState);
      graphRef.current?.refresh?.();

      if (progress < 1) {
        hoverAnimationFrame.current = requestAnimationFrame(tick);
        return;
      }

      const settled: HoverTransition = { from: nextID, to: nextID, progress: 1 };
      hoverTransitionRef.current = settled;
      setHoverTransition(settled);
      graphRef.current?.refresh?.();
      hoverAnimationFrame.current = null;
    };

    hoverAnimationFrame.current = requestAnimationFrame(tick);
  }, []);

  const handleNodeHover = useCallback((node: any) => {
    const nextID = node?.id || null;
    if (hoverClearTimeout.current !== null) {
      window.clearTimeout(hoverClearTimeout.current);
      hoverClearTimeout.current = null;
    }

    if (nextID) {
      startHoverTransition(nextID);
      return;
    }

    hoverClearTimeout.current = window.setTimeout(() => {
      startHoverTransition(null);
      hoverClearTimeout.current = null;
    }, HOVER_CLEAR_DELAY_MS);
  }, [startHoverTransition]);

  const getLinkWidth = useCallback((link: any) => {
    const src = typeof link.source === 'object' ? link.source.id : link.source;
    const tgt = typeof link.target === 'object' ? link.target.id : link.target;
    const fromFocus = getLinkFocus(src, tgt, hoverTransition.from, fromHoveredLinks);
    const toFocus = getLinkFocus(src, tgt, hoverTransition.to, toHoveredLinks);
    const focus = mix(fromFocus, toFocus, hoverTransition.progress);

    if (focus > 0.05) return 1.2 + focus * 1.3;
    return link.type === 'required' ? 1.2 : 0.6;
  }, [hoverTransition, fromHoveredLinks, toHoveredLinks]);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full text-gray-400">
        <div className="w-8 h-8 border-2 border-gray-300 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!filteredGraph || !filteredGraph.nodes || filteredGraph.nodes.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-gray-400">
        <p className="text-sm">No dependency graph data. Scan mods first.</p>
      </div>
    );
  }

  return (
    <div ref={containerRef} className="relative h-full w-full bg-gray-50 overflow-hidden">
      <div className="absolute top-3 right-3 z-10">
        {isSettingsPanelOpen ? (
          <div className="w-[260px] rounded-lg border border-gray-200 bg-white p-3 text-xs shadow-sm">
            <div className="mb-2 flex items-center justify-between gap-3">
              <div className="text-gray-700 font-medium">Settings</div>
              <button
                type="button"
                onClick={() => setIsSettingsPanelOpen(false)}
                className="rounded-md border border-gray-200 px-2 py-1 text-[11px] text-gray-600 hover:bg-gray-50"
              >
                Minify
              </button>
            </div>
            <label className="flex items-start gap-2 pb-2 mb-1 border-b border-gray-200 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={showZoomedLabels}
                onChange={(event) => setShowZoomedLabels(event.target.checked)}
                className="mt-0.5 h-3.5 w-3.5 rounded border-gray-300 text-gray-900 focus:ring-gray-400"
              />
              <span className="leading-4">
                <span className="block text-gray-700">Keep names when zoomed in</span>
                <span className="block text-[10px] text-gray-500">When off, only hovered and connected nodes keep labels up close.</span>
              </span>
            </label>
            <label className="flex items-start gap-2 pb-2 mb-1 border-b border-gray-200 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={hideOptionalMissingNodes}
                onChange={(event) => setHideOptionalMissingNodes(event.target.checked)}
                className="mt-0.5 h-3.5 w-3.5 rounded border-gray-300 text-gray-900 focus:ring-gray-400"
              />
              <span className="leading-4">
                <span className="block text-gray-700">Hide missing optional mods</span>
                <span className="block text-[10px] text-gray-500">Removes optional dependency nodes that are not installed.</span>
              </span>
            </label>
            <label className="flex items-start gap-2 pb-2 mb-1 border-b border-gray-200 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={hideUnconnectedNodes}
                onChange={(event) => setHideUnconnectedNodes(event.target.checked)}
                className="mt-0.5 h-3.5 w-3.5 rounded border-gray-300 text-gray-900 focus:ring-gray-400"
              />
              <span className="leading-4">
                <span className="block text-gray-700">Hide unconnected nodes</span>
                <span className="block text-[10px] text-gray-500">Removes nodes that have no visible links after the current filters are applied.</span>
              </span>
            </label>
            <label className="flex items-start gap-2 pb-2 mb-1 border-b border-gray-200 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={showOptionalConnections}
                onChange={(event) => setShowOptionalConnections(event.target.checked)}
                className="mt-0.5 h-3.5 w-3.5 rounded border-gray-300 text-gray-900 focus:ring-gray-400"
              />
              <span className="leading-4">
                <span className="block text-gray-700">Show optional connections</span>
                <span className="block text-[10px] text-gray-500">Hide dashed optional edges from the graph.</span>
              </span>
            </label>
            {hasFabricAPI && (
              <label className="flex items-start gap-2 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={showFabricAPI}
                  onChange={(event) => setShowFabricAPI(event.target.checked)}
                  className="mt-0.5 h-3.5 w-3.5 rounded border-gray-300 text-gray-900 focus:ring-gray-400"
                />
                <span className="leading-4">
                  <span className="block text-gray-700">Show Fabric API</span>
                  <span className="block text-[10px] text-gray-500">Hide the common Fabric API hub node to reduce graph noise.</span>
                </span>
              </label>
            )}
          </div>
        ) : (
          <div className="rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs shadow-sm">
            <div className="flex items-center gap-3">
              <div className="text-gray-700 font-medium">Settings</div>
              <button
                type="button"
                onClick={() => setIsSettingsPanelOpen(true)}
                className="rounded-md border border-gray-200 px-2 py-1 text-[11px] text-gray-600 hover:bg-gray-50"
              >
                Open
              </button>
            </div>
          </div>
        )}
      </div>

      <div className="absolute bottom-3 left-3 z-10 rounded-lg border border-gray-200 bg-white p-3 text-xs space-y-1.5 shadow-sm">
        <div className="text-gray-700 font-medium mb-1">Legend</div>
        <div className="flex items-center gap-2">
          <div className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: LOADER_COLORS.fabric }} />
          <span className="text-gray-700">Fabric</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: LOADER_COLORS.forge }} />
          <span className="text-gray-700">Forge / NeoForge</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: GROUP_COLORS.normal }} />
          <span className="text-gray-700">Unknown loader</span>
        </div>
        <hr className="border-gray-200" />
        <div className="flex items-center gap-2">
          <div className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: GROUP_COLORS.library }} />
          <span className="text-gray-700">Library</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: GROUP_COLORS.unused }} />
          <span className="text-gray-700">Unused library</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-2.5 h-2.5 rounded-full" style={{ backgroundColor: GROUP_COLORS.missing }} />
          <span className="text-gray-700">Missing dependency</span>
        </div>
        <hr className="border-gray-200" />
        <div className="text-gray-400 flex items-center gap-2">
          <svg width="24" height="8"><line x1="0" y1="4" x2="16" y2="4" stroke="#6b7280" strokeWidth="2"/><polygon points="16,0 24,4 16,8" fill="#6b7280"/></svg>
          <span>Required</span>
        </div>
        <div className="text-gray-400 flex items-center gap-2">
          <svg width="24" height="8"><line x1="0" y1="4" x2="16" y2="4" stroke="#9ca3af" strokeWidth="1.5" strokeDasharray="3,3"/><polygon points="16,1 23,4 16,7" fill="#9ca3af"/></svg>
          <span>Optional</span>
        </div>
      </div>

      {/* Controls */}
      <div className="absolute bottom-3 right-3 z-10 flex gap-1">
        <button
          onClick={() => graphRef.current?.zoomToFit(400, 60)}
          className="px-2.5 py-1.5 rounded-lg bg-white border border-gray-200 text-gray-600 text-xs hover:bg-gray-50 shadow-sm"
        >
          Fit View
        </button>
      </div>

      {/* Node count */}
      <div className="absolute top-3 left-3 z-10 text-xs text-gray-600">
        {filteredGraph.nodes.length} nodes · {filteredGraph.links.length} links
      </div>

      <div className="absolute inset-0">
        {dimensions.width > 0 && dimensions.height > 0 && (
          <ForceGraph2D
            key={`${dimensions.width}x${dimensions.height}-${filteredGraph.nodes.length}-${filteredGraph.links.length}-${showOptionalConnections ? 'opt' : 'req'}-${hideOptionalMissingNodes ? 'hide-missing' : 'show-missing'}-${hideUnconnectedNodes ? 'hide-unconnected' : 'show-unconnected'}-${showFabricAPI ? 'show-fabric-api' : 'hide-fabric-api'}`}
            ref={graphRef}
            graphData={filteredGraph}
            nodeCanvasObject={paintNode}
            nodePointerAreaPaint={nodePointerAreaPaint}
            linkColor={getLinkColor}
            linkWidth={getLinkWidth}
            linkCurvature={(link: any) => {
              const source = typeof link.source === 'object' ? link.source.id : link.source;
              const target = typeof link.target === 'object' ? link.target.id : link.target;
              return linkCurvature.get(getUndirectedLinkKey(source, target)) ?? 0;
            }}
            linkDirectionalArrowLength={8}
            linkDirectionalArrowRelPos={0.95}
            linkDirectionalArrowColor={getLinkColor}
            linkLineDash={(link: any) => link.type === 'optional' ? [4, 4] : null}
            onNodeClick={(node: any) => node?.id && onSelectMod(node.id)}
            onNodeHover={handleNodeHover}
            backgroundColor="transparent"
            width={dimensions.width}
            height={dimensions.height}
            warmupTicks={100}
            cooldownTicks={400}
            nodeLabel=""
          />
        )}
      </div>
    </div>
  );
}
