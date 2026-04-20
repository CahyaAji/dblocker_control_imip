<script lang="ts">
    import { onMount } from "svelte";
    import maplibregl from "maplibre-gl";
    import "maplibre-gl/dist/maplibre-gl.css";
    import { settings } from "../store/configStore";
    import { dblockerStore, expandedDblockerId, type DBlocker } from "../store/dblockerStore";
    import { bridgeStore } from "../store/bridgeStore";
    import {
        detectorStore,
        fetchDetectors,
        type DroneDetector,
    } from "../store/detectorStore";

    let mapContainer: HTMLElement;
    let map = $state<maplibregl.Map>();

    // Track HTML overlay markers (for radar animation only)
    let overlayMarkers = new Map<number, maplibregl.Marker>();
    let previousConfigMap = new Map<number, string>();

    let resizeObserver: ResizeObserver;
    let debounceTimer: ReturnType<typeof setTimeout>;

    const SOURCE_ID = "dblockers-source";
    const LAYER_GLOW_ID = "dblockers-glow";
    const LAYER_CORE_ID = "dblockers-core";
    const LAYER_BORDER_ID = "dblockers-border";
    const LAYER_LABEL_ID = "dblockers-label";

    const DET_SOURCE_ID = "detectors-source";
    const DET_LAYER_GLOW_ID = "detectors-glow";
    const DET_LAYER_BORDER_ID = "detectors-border";
    const DET_LAYER_CORE_ID = "detectors-core";
    const DET_LAYER_LABEL_ID = "detectors-label";

    const MAP_STYLES = {
        normal: "https://api.maptiler.com/maps/openstreetmap/style.json?key=aUOEn1bA48mz3xc3pL4N",
        hybrid: "https://api.maptiler.com/maps/hybrid/style.json?key=aUOEn1bA48mz3xc3pL4N",
    };

    $effect(() => {
        if (map) {
            const expandedId = $expandedDblockerId;
            overlayMarkers.forEach((marker, id) => {
                marker.getElement().classList.toggle("show-sector-labels", id === expandedId);
            });
        }
    });

    $effect(() => {
        if (map && $dblockerStore.length > 0) {
            // Also depend on bridgeStore so markers update when status changes
            const _ = $bridgeStore;
            debounceRender($dblockerStore);
        }
    });

    $effect(() => {
        if (map && $detectorStore.length > 0)
            updateDetectorMarkers($detectorStore);
    });

    $effect(() => {
        if (map && $settings.mapStyle) {
            map.setStyle(MAP_STYLES[$settings.mapStyle]);
            // Re-add source/layers after style change
            map.once("style.load", () => {
                addSourceAndLayers();
                addDetectorSourceAndLayers();
                if ($dblockerStore.length > 0) updateMarkers($dblockerStore);
                if ($detectorStore.length > 0)
                    updateDetectorMarkers($detectorStore);
            });
        }
    });

    function addSourceAndLayers() {
        if (!map || map.getSource(SOURCE_ID)) return;

        map.addSource(SOURCE_ID, {
            type: "geojson",
            data: { type: "FeatureCollection", features: [] },
        });

        // Outer glow ring
        map.addLayer({
            id: LAYER_GLOW_ID,
            type: "circle",
            source: SOURCE_ID,
            paint: {
                "circle-radius": 12,
                "circle-color": "rgba(210, 54, 31, 0.15)",
                "circle-blur": 0.5,
            },
        });

        // White border ring
        map.addLayer({
            id: LAYER_BORDER_ID,
            type: "circle",
            source: SOURCE_ID,
            paint: {
                "circle-radius": 9,
                "circle-color": "rgba(255, 255, 255, 0.95)",
            },
        });

        // Core red dot
        map.addLayer({
            id: LAYER_CORE_ID,
            type: "circle",
            source: SOURCE_ID,
            paint: {
                "circle-radius": 7,
                "circle-color": "#ff6f59",
            },
        });

        // Name label
        map.addLayer({
            id: LAYER_LABEL_ID,
            type: "symbol",
            source: SOURCE_ID,
            layout: {
                "text-field": ["get", "name"],
                "text-size": 12,
                "text-anchor": "top",
                "text-offset": [0, 1.2],
                "text-font": ["Open Sans Bold", "Arial Unicode MS Bold"],
            },
            paint: {
                "text-color": "#ffffff",
                "text-halo-color": "rgba(0, 0, 0, 0.7)",
                "text-halo-width": 1.5,
            },
        });
    }

    function addDetectorSourceAndLayers() {
        if (!map || map.getSource(DET_SOURCE_ID)) return;

        map.addSource(DET_SOURCE_ID, {
            type: "geojson",
            data: { type: "FeatureCollection", features: [] },
        });

        map.addLayer({
            id: DET_LAYER_GLOW_ID,
            type: "circle",
            source: DET_SOURCE_ID,
            paint: {
                "circle-radius": 14,
                "circle-color": "rgba(0, 200, 255, 0.15)",
                "circle-blur": 0.5,
            },
        });

        map.addLayer({
            id: DET_LAYER_BORDER_ID,
            type: "circle",
            source: DET_SOURCE_ID,
            paint: {
                "circle-radius": 10,
                "circle-color": "rgba(255, 255, 255, 0.95)",
            },
        });

        map.addLayer({
            id: DET_LAYER_CORE_ID,
            type: "circle",
            source: DET_SOURCE_ID,
            paint: {
                "circle-radius": 8,
                "circle-color": [
                    "case",
                    ["==", ["get", "status"], "online"],
                    "#22c55e",
                    "#666666",
                ],
            },
        });

        map.addLayer({
            id: DET_LAYER_LABEL_ID,
            type: "symbol",
            source: DET_SOURCE_ID,
            layout: {
                "text-field": [
                    "concat",
                    ["get", "name"],
                    " (",
                    ["get", "status"],
                    ")",
                ],
                "text-size": 12,
                "text-anchor": "top",
                "text-offset": [0, 1.4],
                "text-font": ["Open Sans Bold", "Arial Unicode MS Bold"],
            },
            paint: {
                "text-color": [
                    "case",
                    ["==", ["get", "status"], "online"],
                    "#22c55e",
                    "#999999",
                ],
                "text-halo-color": "rgba(0, 0, 0, 0.8)",
                "text-halo-width": 1.5,
            },
        });
    }

    function buildDetectorGeoJSON(
        data: DroneDetector[],
    ): GeoJSON.FeatureCollection {
        return {
            type: "FeatureCollection",
            features: data
                .filter((d) => d.latitude != null && d.longitude != null)
                .map((d) => ({
                    type: "Feature" as const,
                    geometry: {
                        type: "Point" as const,
                        coordinates: [d.longitude, d.latitude],
                    },
                    properties: {
                        id: d.id,
                        name: d.name,
                        status: d.status || "offline",
                    },
                })),
        };
    }

    function updateDetectorMarkers(data: DroneDetector[]) {
        if (!map) return;
        const source = map.getSource(DET_SOURCE_ID) as maplibregl.GeoJSONSource;
        if (source) {
            source.setData(buildDetectorGeoJSON(data));
        }
    }

    function buildGeoJSON(data: DBlocker[]): GeoJSON.FeatureCollection {
        return {
            type: "FeatureCollection",
            features: data
                .filter((d) => d.latitude != null && d.longitude != null)
                .map((d) => ({
                    type: "Feature" as const,
                    geometry: {
                        type: "Point" as const,
                        coordinates: [d.longitude!, d.latitude!],
                    },
                    properties: {
                        id: d.id,
                        name: d.name,
                    },
                })),
        };
    }

    function updateMarkers(data: DBlocker[]) {
        if (!map) return;

        // 1. Update native GeoJSON source (perfectly synced with tiles)
        const source = map.getSource(SOURCE_ID) as maplibregl.GeoJSONSource;
        if (source) {
            source.setData(buildGeoJSON(data));
        }

        // 2. Update HTML overlay markers (radar animation only)
        const incomingIds = new Set(data.map((d) => d.id));

        // Cleanup removed
        for (const [id, marker] of overlayMarkers) {
            if (!incomingIds.has(id)) {
                marker.remove();
                overlayMarkers.delete(id);
                previousConfigMap.delete(id);
            }
        }

        data.forEach((dblocker) => {
            if (dblocker.latitude == null || dblocker.longitude == null) return;

            const { id, serial_numb, latitude, longitude, ...visualData } =
                dblocker;
            const staTopic = `dbl/${dblocker.serial_numb}/sta`;
            const staPayload = $bridgeStore[staTopic] ?? null;
            const isOn = staPayload !== null && staPayload.startsWith('ON');
            const currentConfigSig = JSON.stringify({ ...visualData, isOn });
            const prevConfigSig = previousConfigMap.get(dblocker.id);
            const hasMarker = overlayMarkers.has(dblocker.id);

            if (!hasMarker || currentConfigSig !== prevConfigSig) {
                if (hasMarker) overlayMarkers.get(dblocker.id)?.remove();

                const el = createRadarElement(dblocker, isOn);
                // Immediately apply show-sector-labels if this is the expanded marker
                if ($expandedDblockerId === dblocker.id) {
                    el.classList.add("show-sector-labels");
                }
                const marker = new maplibregl.Marker({
                    element: el,
                    anchor: "center",
                })
                    .setLngLat([dblocker.longitude, dblocker.latitude])
                    .addTo(map!);

                overlayMarkers.set(dblocker.id, marker);
                previousConfigMap.set(dblocker.id, currentConfigSig);
            } else if (hasMarker) {
                overlayMarkers
                    .get(dblocker.id)
                    ?.setLngLat([dblocker.longitude, dblocker.latitude]);
            }
        });
    }

    function createRadarElement(dblocker: DBlocker, isOn: boolean) {
        // Zero-size container — all children use absolute positioning
        const el = document.createElement("div");
        el.className = "marker-radar";
        const baseRotation = dblocker.angle_start || 0;
        const configs = dblocker.config || [];
        for (let i = 0; i < 6; i++) {
            const angle = i * 60 + baseRotation - 90;

            // Only render radar slices if dblocker is ON
            if (isOn) {
                for (let layer = 0; layer < 2; layer++) {
                    const sectorConfig = configs[i];
                    if (!sectorConfig) continue;

                    if (sectorConfig.signal_ctrl === false && layer === 0) continue;
                    if (sectorConfig.signal_gps === false && layer === 1) continue;

                    for (let ripple = 0; ripple < 2; ripple++) {
                        const slice = document.createElement("div");
                        slice.className = "radar-slice";

                        slice.style.setProperty("--angle", `${angle}deg`);
                        slice.style.setProperty(
                            "--color",
                            layer === 1 ? "darkgreen" : "yellow",
                        );

                        const scaleWrapper = layer === 1 ? 0.6 : 1.0;
                        slice.style.setProperty(
                            "--scale-factor",
                            `${scaleWrapper}`,
                        );
                        slice.style.animationDelay = `${ripple * -1}s`;

                        el.appendChild(slice);
                    }
                }
            }

            // Sector number label
            const label = document.createElement("div");
            label.className = "sector-label";
            label.textContent = `${i + 1}`;
            const labelRadius = 40;
            const rad = (angle * Math.PI) / 180;
            label.style.left = `${Math.cos(rad) * labelRadius}px`;
            label.style.top = `${Math.sin(rad) * labelRadius}px`;
            el.appendChild(label);
        }

        return el;
    }

    function debounceRender(data: DBlocker[]) {
        clearTimeout(debounceTimer);
        debounceTimer = setTimeout(() => {
            updateMarkers(data);
        }, 100);
    }

    function switchStyle(styleKey: "normal" | "hybrid") {
        $settings.mapStyle = styleKey;
    }

    onMount(() => {
        map = new maplibregl.Map({
            container: mapContainer,
            style: MAP_STYLES[$settings.mapStyle],
            center: [122.13, -2.81],
            zoom: 13,
        });
        map.addControl(new maplibregl.NavigationControl(), "top-left");

        map.on("styleimagemissing", (e) => {
            if (e.id === " " || e.id === "null") {
                const width = 1;
                const height = 1;
                const data = new Uint8Array(width * height * 4);
                map?.addImage(e.id, { width, height, data });
            }
        });

        resizeObserver = new ResizeObserver(() => {
            map?.resize();
        });
        resizeObserver.observe(mapContainer);

        map.on("load", () => {
            addSourceAndLayers();
            addDetectorSourceAndLayers();
            fetchDetectors();

            console.log(
                "DBlocker Store on map load: " + JSON.stringify($dblockerStore),
            );
            if ($dblockerStore.length > 0) {
                updateMarkers($dblockerStore);
            }
        });

        return () => {
            resizeObserver?.disconnect();
            overlayMarkers.forEach((m) => m.remove());
            overlayMarkers.clear();
            previousConfigMap.clear();

            if (map) {
                map.remove();
            }
        };
    });
</script>

<div class="map-layout">
    <div class="map-buttons">
        <button
            class:active={$settings.mapStyle === "normal"}
            onclick={() => switchStyle("normal")}>Normal</button
        >
        <button
            class:active={$settings.mapStyle === "hybrid"}
            onclick={() => switchStyle("hybrid")}>Satellite</button
        >
    </div>
    <div class="map-container" bind:this={mapContainer}></div>
</div>

<style>
    .map-layout {
        height: 100%;
        display: flex;
        flex-direction: column;
        overflow: hidden;
        border-radius: 0;
    }

    .map-buttons {
        margin-left: 48px;
        margin-top: 14px;
        padding: 4px;
        display: flex;
        gap: 6px;
        position: absolute;
        z-index: 2;
        border-radius: 999px;
        border: 1px solid color-mix(in srgb, var(--separator) 78%, transparent);
        background: color-mix(in srgb, var(--card-bg) 84%, transparent);
        backdrop-filter: blur(8px);
    }

    button {
        padding: 7px 14px;
        border-radius: 999px;
        border: none;
        background-color: transparent;
        color: var(--text-secondary);
        font-size: 12px;
        font-weight: 700;
        cursor: pointer;
        transition: all 0.2s ease;
    }

    button:hover {
        color: var(--text-primary);
    }

    button.active {
        background: var(--card-bg);
        color: var(--text-primary);
        box-shadow: 0 6px 16px rgba(18, 35, 48, 0.15);
    }

    .map-container {
        flex-grow: 1;
    }

    /* Zero-size container so MapLibre anchor is always at the exact coordinate */
    .map-layout :global(.marker-radar) {
        width: 0;
        height: 0;
        position: relative;
    }

    .map-layout :global(.radar-slice) {
        position: absolute;
        width: calc(120px * var(--scale-factor));
        height: calc(120px * var(--scale-factor));
        background-color: var(--color);

        /* Center on the zero-size parent */
        top: 0;
        left: 0;
        transform: translate(-50%, -50%) rotate(var(--angle));
        clip-path: polygon(50% 50%, 100% 20%, 100% 80%);
        border-radius: 50%;

        pointer-events: none;

        animation: zoom-pulse 2s infinite linear;
    }

    .map-layout :global(.sector-label) {
        position: absolute;
        transform: translate(-50%, -50%);
        font-size: 14px;
        font-weight: 700;
        color: #fff;
        background: rgba(0, 0, 0, 0.75);
        width: 22px;
        height: 22px;
        line-height: 22px;
        text-align: center;
        border-radius: 50%;
        pointer-events: none;
        z-index: 1;
        display: none;
        box-shadow: 0 2px 8px rgba(0,0,0,0.18);
        border: 1.5px solid #fff2;
    }

    .map-layout :global(.show-sector-labels .sector-label) {
        display: block;
    }

    @keyframes zoom-pulse {
        0% {
            transform: translate(-50%, -50%) rotate(var(--angle)) scale(0);
            opacity: 0.8;
        }
        30% {
            transform: translate(-50%, -50%) rotate(var(--angle)) scale(0.3);
            opacity: 1;
        }

        100% {
            transform: translate(-50%, -50%) rotate(var(--angle)) scale(1);
            opacity: 0;
        }
    }
</style>
