<script lang="ts">
    import { onMount } from "svelte";
    import maplibregl from "maplibre-gl";
    import "maplibre-gl/dist/maplibre-gl.css";
    import { settings } from "../store/configStore";
    import { dblockerStore, type DBlocker } from "../store/dblockerStore";

    let mapContainer: HTMLElement;
    let map = $state<maplibregl.Map>();

    // Track Markers
    let markers = new Map<number, maplibregl.Marker>();
    // Track Config State (to detect changes)
    let previousConfigMap = new Map<number, string>();

    let resizeObserver: ResizeObserver;
    let debounceTimer: ReturnType<typeof setTimeout>;

    const MAP_STYLES = {
        normal: "https://api.maptiler.com/maps/openstreetmap/style.json?key=fB2eDjoDg2nlel5Kw6ym",
        hybrid: "https://api.maptiler.com/maps/hybrid/style.json?key=aUOEn1bA48mz3xc3pL4N",
    };

    // ✨ 1. REACTIVE DATA LISTENER
    // Whenever $dblockerStore changes (polling or user click), this runs.
    $effect(() => {
        if (map && $dblockerStore.length > 0) debounceRender($dblockerStore);
    });

    // ✨ 2. REACTIVE STYLE LISTENER
    // Whenever $settings changes, this runs.
    $effect(() => {
        if (map && $settings.mapStyle)
            map.setStyle(MAP_STYLES[$settings.mapStyle]);
    });

    function updateMarkers(data: DBlocker[]) {
        if (!map) return;
        const incomingIds = new Set(data.map((loc) => loc.id));

        // Cleanup removed markers
        for (const [id, marker] of markers) {
            if (!incomingIds.has(id)) {
                marker.remove();
                markers.delete(id);
                previousConfigMap.delete(id);
            }
        }

        // Add/Update markers
        data.forEach((dblocker) => {
            if (dblocker.latitude == null || dblocker.longitude == null) return;

            // 1. Identify constant fields (id, serial_numb) vs changeable fields
            const { id, serial_numb, ...changeableData } = dblocker;
            // 2. Separate position from other visual config to optimize updates
            const { latitude, longitude, ...visualData } = changeableData;
            const currentConfigSig = JSON.stringify(visualData);
            const prevConfigSig = previousConfigMap.get(dblocker.id);
            const hasMarker = markers.has(dblocker.id);

            // CASE 1: Config Changed OR New Marker -> Full Re-Render
            if (!hasMarker || currentConfigSig !== prevConfigSig) {
                // If it existed but config changed (e.g. user flipped switch), remove old one
                if (hasMarker) markers.get(dblocker.id)?.remove();

                const el = createMarkerElement(dblocker);
                const newMarker = new maplibregl.Marker({ element: el })
                    .setLngLat([dblocker.longitude, dblocker.latitude])
                    .addTo(map!);

                markers.set(dblocker.id, newMarker);
                previousConfigMap.set(dblocker.id, currentConfigSig);
            }
            // CASE 2: Config is same, just move it (Performance optimization)
            else if (hasMarker) {
                markers
                    .get(dblocker.id)
                    ?.setLngLat([dblocker.longitude, dblocker.latitude]);
            }
        });
    }

    function createMarkerElement(dblocker: DBlocker) {
        const el = document.createElement("div");
        el.className = "marker-gps";
        const baseRotation = dblocker.angle_start || 0;
        const configs = dblocker.config || [];
        for (let i = 0; i < 6; i++) {
            const angle = i * 60 + baseRotation;
            for (let layer = 0; layer < 2; layer++) {
                const sectorConfig = configs[i];
                if (!sectorConfig) continue;

                if (sectorConfig.signal_ctrl === false && layer === 0) continue;
                if (sectorConfig.signal_gps === false && layer === 1) continue;

                // Create 2 ripples per sector for the overlapping effect
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

                    // Delay the second ripple by -1s so it starts halfway through
                    slice.style.animationDelay = `${ripple * -1}s`;

                    el.appendChild(slice);
                }
            }
        }

        const core = document.createElement("div");
        core.className = "marker-core";
        el.appendChild(core);

        return el;
    }

    function updatePixelScale() {
        if (!map || !mapContainer) return;

        const zoom = map.getZoom();
        const lat = map.getCenter().lat;

        // Math to convert meters to pixels at current zoom level
        const metersPerPixel =
            (156543.03392 * Math.cos((lat * Math.PI) / 180)) /
            Math.pow(2, zoom);

        // 1km Radius = 2000m Diameter
        const diameterInMeters = 2000;

        const pixels = diameterInMeters / metersPerPixel;

        // Update the CSS variable
        mapContainer.style.setProperty("--px-diameter", `${pixels}px`);
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
        // ✨ Initialize using Store value
        map = new maplibregl.Map({
            container: mapContainer,
            style: MAP_STYLES[$settings.mapStyle],
            center: [110.44053927286228, -7.777395993083473],
            zoom: 14,
        });
        map.addControl(new maplibregl.NavigationControl(), "top-left");

        // Fix for "Image " " could not be loaded" error from MapTiler style
        map.on("styleimagemissing", (e) => {
            if (e.id === " " || e.id === "null") {
                const width = 1;
                const height = 1;
                const data = new Uint8Array(width * height * 4);
                map?.addImage(e.id, { width, height, data });
            }
        });

        // ✨ Resize Observer: Watches for sidebar changes
        resizeObserver = new ResizeObserver(() => {
            map?.resize();
        });
        resizeObserver.observe(mapContainer);

        map.on("load", () => {
            updatePixelScale();

            if (map) {
                map.on("move", updatePixelScale);
                map.on("zoom", updatePixelScale);
            }

            // Initial render if data is already in store
            console.log(
                "DBlocker Store on map load: " + JSON.stringify($dblockerStore),
            );
            if ($dblockerStore.length > 0) {
                updateMarkers($dblockerStore);
            }
        });

        return () => {
            resizeObserver?.disconnect();
            markers.forEach((m) => m.remove());
            markers.clear();
            previousConfigMap.clear();

            if (map) {
                map.off("move", updatePixelScale);
                map.off("zoom", updatePixelScale);
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
        border: 1px solid #ccc;
    }

    .map-buttons {
        margin-left: 50px;
        margin-top: 10px;
        display: flex;
        gap: 10px;
        position: absolute;
        z-index: 2;
    }

    button {
        padding: 6px 12px;
        border-radius: 6px;
        border: solid 1px #ccc;
        background-color: white;
        cursor: pointer;
    }

    button.active {
        background-color: #333;
        color: white;
        border-color: #333;
    }

    .map-container {
        flex-grow: 1;
    }

    .map-layout :global(.marker-gps) {
        display: flex;
        align-items: center;
        justify-content: center;
        position: relative;
    }

    .map-layout :global(.marker-core) {
        width: 18px;
        height: 18px;
        background: red;
        border: 2px solid white;
        border-radius: 50%;
        z-index: 2;
        position: relative;
    }

    .map-layout :global(.radar-slice) {
        position: absolute;
        width: calc(var(--px-diameter) * var(--scale-factor));
        height: calc(var(--px-diameter) * var(--scale-factor));
        background-color: var(--color);

        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%) rotate(var(--angle));
        clip-path: polygon(50% 50%, 100% 20%, 100% 80%);
        border-radius: 50%;

        pointer-events: none;

        /* Linear makes the ripple overlap smoothly */
        animation: zoom-pulse 2s infinite linear;
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
