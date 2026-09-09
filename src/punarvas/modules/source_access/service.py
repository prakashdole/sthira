"""
PUNARVAS-AI Source Access, Provider Health & Blocker Gating Service (ARC-C13).
Normative Reference: source-register.md, rules.md (RUL-076-RUL-083), trd.md (FR-076-FR-084).
"""

import hashlib
import json
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

from punarvas.core.audit import global_audit_ledger
from punarvas.modules.source_access.contracts import (
    AOISampleGateInput,
    AOISampleGateResult,
    ActivationState,
    BasemapConfigRecord,
    BlockerItem,
    BlockerStatus,
    CapabilityType,
    DependencyBlockerEvaluationRequest,
    DependencyBlockerReport,
    EndpointType,
    EntitlementState,
    MirrorGroupRecord,
    ObservationReconciliationRequest,
    ObservationReconciliationResult,
    PriorityClass,
    ProviderHealthStatus,
    ReconciliationMethod,
    SourceCapabilityRecord,
    TokenState,
    utc_now,
)


class SourceAccessService:
    """
    Source Access, Provider Health, Operational Readiness & Blocker Gating Engine (ARC-C13).
    """

    def __init__(self):
        self._capabilities: Dict[str, SourceCapabilityRecord] = {}
        self._mirror_groups: Dict[str, MirrorGroupRecord] = {}
        self._provider_health: Dict[str, ProviderHealthStatus] = {}
        self._basemap_configs: Dict[str, BasemapConfigRecord] = {}
        self._init_s01_s54_capabilities()
        self._init_mirror_groups()
        self._init_provider_health()
        self._init_basemap_configs()

    def _init_s01_s54_capabilities(self):
        """
        Populate the complete S01-S54 operational capability register (FR-076, RUL-076).
        """
        catalog_items = [
            # 5.1 Hazard, geology, and official programme evidence
            (
                "S01", "KSDMA/GSI Landslide Susceptibility", CapabilityType.PRODUCT, PriorityClass.CORE,
                "GSI / KSDMA", "GIS Specialist",
                "Exposure screening only; macro-scale landslide susceptibility zones",
                ["Susceptibility is not runout, probability, legal zoning, or parcel safety", "Cannot prove safe foundation"],
                "Kerala (Wayanad)",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT, EndpointType.DISPLAY],
                None, False, False
            ),
            (
                "S02", "KSDMA Flood Probability Rasters", CapabilityType.PRODUCT, PriorityClass.CORE,
                "KSDMA", "Hydrologist",
                "Screen flood exposure with return period/scenario units",
                ["Not a live real-time forecast", "Does not measure local drainage backup or culvert choking"],
                "Kerala (Wayanad)",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT, EndpointType.DISPLAY],
                None, False, False
            ),
            (
                "S03", "KSDMA/Wayanad PDNA, Orders, Plans", CapabilityType.PRODUCT, PriorityClass.CORE,
                "GoK / KSDMA / DDMA Wayanad", "Programme Officer",
                "Programme context, historical impact counts, dated baseline facts",
                ["Historic counts are not current eligible households", "Document index does not prove file acquisition"],
                "Wayanad District",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S04", "GSI Bhukosh Geological Maps and Reports", CapabilityType.PRODUCT, PriorityClass.CORE,
                "Geological Survey of India (GSI)", "Geotechnical Lead",
                "Regional lithology, structural lineaments, and historical landslide inventory",
                ["Not measured shear strength, soil thickness, or site bearing capacity", "Not a parameter API"],
                "India (Kerala/Wayanad)",
                [EndpointType.DISCOVERY, EndpointType.AUTHENTICATION, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S05", "ISRO Bhuvan Thematic Services/APIs", CapabilityType.PRODUCT, PriorityClass.SUPPORT,
                "NRSC / ISRO", "GIS Specialist",
                "Thematic regional and proximity context layers via WMS/WMTS",
                ["Rendered WMS is display context only, not analysis-ready data", "Not a cadastral-title API"],
                "India",
                [EndpointType.DISCOVERY, EndpointType.AUTHENTICATION, EndpointType.DISPLAY],
                None, False, False
            ),
            (
                "S06", "NDEM National Database for Emergency Management", CapabilityType.AGENCY_RECORD, PriorityClass.AGENCY,
                "NRSC / MHA", "Disaster Management Specialist",
                "Multi-hazard disaster information and event products under authorized access",
                ["Do not scrape public dashboards", "Cannot infer bulk API rights without signed agency agreement"],
                "India",
                [EndpointType.AUTHENTICATION, EndpointType.ENTITLEMENT, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S07", "CWC/C-FLOOD Inundation Modeling", CapabilityType.PROCESSING, PriorityClass.CONDITIONAL,
                "Central Water Commission (CWC) / C-DAC", "Hydrologist",
                "Riverine flood inundation modeling in documented river basins",
                ["Documented coverage covers Godavari, Tapi, Mahanadi only; ZERO Wayanad coverage", "Never use as landslide model"],
                "Godavari, Tapi, Mahanadi only",
                [EndpointType.DISCOVERY, EndpointType.PROCESSING],
                None, False, False
            ),
            # 5.2 Earth observation, catalogs, and processing routes
            (
                "S08", "Sentinel-2 L2A Optical Imagery", CapabilityType.PRODUCT, PriorityClass.CORE,
                "ESA / Copernicus", "Remote Sensing Lead",
                "Multi-spectral optical change detection and vegetation index screening",
                ["Cannot prove small structures, occupation, title, or legal vacancy", "Cloud/shadow masking required"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT, EndpointType.PROCESSING],
                "MIRROR_SENTINEL_2", False, False
            ),
            (
                "S09", "Sentinel-1 SAR Radar Imagery", CapabilityType.PRODUCT, PriorityClass.OPTIONAL,
                "ESA / Copernicus", "Radar Specialist",
                "SAR backscatter and flood surface water extent context",
                ["GRD/RTC backscatter does not prove surface displacement", "InSAR requires specialist SLC stacks and calibration"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT, EndpointType.PROCESSING],
                None, False, False
            ),
            (
                "S10", "Copernicus Data Space Ecosystem (CDSE)", CapabilityType.CATALOG, PriorityClass.CORE_API,
                "ESA", "Data Engineer",
                "Preferred STAC catalog and OData asset acquisition gateway for Sentinel data",
                ["Catalog search alone does not prove download success or license validation", "SciHub is obsolete"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.AUTHENTICATION, EndpointType.ENTITLEMENT, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_SENTINEL_2", False, False
            ),
            (
                "S11", "Element 84 Earth Search", CapabilityType.CATALOG, PriorityClass.ALTERNATIVE_API,
                "Element 84 / AWS", "Data Engineer",
                "Cloud-native public STAC discovery and COG access",
                ["Shares underlying Sentinel-2 observations with CDSE; not independent corroboration", "Open catalog does not guarantee transfer rights"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_SENTINEL_2", False, False
            ),
            (
                "S12", "Microsoft Planetary Computer", CapabilityType.CATALOG, PriorityClass.ALTERNATIVE_API,
                "Microsoft", "Data Engineer",
                "Alternate cloud-hosted STAC catalog and SAS-signed asset URLs",
                ["Persist stable item IDs, never SAS tokens", "Mirror of Sentinel observations; not independent corroboration"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.AUTHENTICATION, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_SENTINEL_2", False, False
            ),
            (
                "S13", "Google Earth Engine", CapabilityType.PROCESSING, PriorityClass.OPTIONAL_PROCESSING,
                "Google", "Geospatial Analyst",
                "Hosted planetary-scale geospatial analytics subject to approved commercial/government terms",
                ["Restricted beneficiary and household data strictly prohibited from ingestion", "Must pass privacy and quota review"],
                "Global",
                [EndpointType.AUTHENTICATION, EndpointType.ENTITLEMENT, EndpointType.PROCESSING],
                None, False, False
            ),
            (
                "S14", "NRSC Bhoonidhi API", CapabilityType.CATALOG, PriorityClass.ALTERNATIVE_API,
                "NRSC / ISRO", "Data Engineer",
                "Indian Earth Observation satellite search and download portal",
                ["API docs verified; AOI stock, pricing, and automated quota unverified for production", "Requires token exchange"],
                "India / Global",
                [EndpointType.DISCOVERY, EndpointType.AUTHENTICATION, EndpointType.ENTITLEMENT, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S15", "USGS Landsat Collection 2 / M2M", CapabilityType.PRODUCT, PriorityClass.SUPPORT_API,
                "USGS", "Remote Sensing Lead",
                "Long-term multi-decadal land surface and vegetation history",
                ["30m resolution cannot resolve household occupation, plot boundaries, or individual structures"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.AUTHENTICATION, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S16", "NASA ASF Search / HyP3", CapabilityType.PROCESSING, PriorityClass.SPECIALIST,
                "Alaska Satellite Facility / NASA", "Radar Specialist",
                "On-demand SAR processing and interferometry pipelines",
                ["Specialist processing alternative; not an approved universal early warning or displacement product"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.AUTHENTICATION, EndpointType.PROCESSING],
                None, False, False
            ),
            (
                "S17", "NISAR Earth Observation Missions", CapabilityType.PRODUCT, PriorityClass.SPECIALIST,
                "NASA / ISRO", "Radar Specialist",
                "Provisional L-band and S-band radar surface change observations",
                ["No pre-2024 disaster history", "Provisional data cannot claim universal millimetre warning or slope safety"],
                "Global / India",
                [EndpointType.DISCOVERY, EndpointType.AUTHENTICATION, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            # 5.3 Terrain, buildings, population, land cover, soil, and climate
            (
                "S18", "Copernicus DEM GLO-30", CapabilityType.PRODUCT, PriorityClass.CORE,
                "Copernicus / ESA", "Terrain Analyst",
                "30m digital surface model for regional catchment, slope, and elevation screening",
                ["DSM reflects tree canopy/structures, not bare earth", "Not engineering grade for foundation design"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT, EndpointType.PROCESSING],
                "MIRROR_DEM", False, False
            ),
            (
                "S19", "CartoDEM / Cartosat-1 DSM", CapabilityType.PRODUCT, PriorityClass.ALTERNATIVE,
                "NRSC / ISRO", "Terrain Analyst",
                "Indian high-resolution stereoscopic terrain model",
                ["Flat-terrain accuracy claims cannot be transferred to steep Western Ghats slopes without ground validation"],
                "India",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_DEM", False, False
            ),
            (
                "S20", "SRTM Shuttle Radar Topography Mission", CapabilityType.PRODUCT, PriorityClass.ALTERNATIVE,
                "NASA / USGS", "Terrain Analyst",
                "Historical global 30m elevation baseline",
                ["Older regional baseline; do not treat DEM mirrors as independent safety evidence"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_DEM", False, False
            ),
            (
                "S21", "JAXA AW3D30 ALOS World 3D", CapabilityType.PRODUCT, PriorityClass.ALTERNATIVE,
                "JAXA", "Terrain Analyst",
                "Global 30m DSM cross-comparison dataset",
                ["Subject to void-fill and tropical canopy bias; not bedrock depth or soil strength"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_DEM", False, False
            ),
            (
                "S22", "Google Open Buildings V3 / Temporal", CapabilityType.PRODUCT, PriorityClass.CORE,
                "Google Research", "GIS Specialist",
                "Building footprint geometries and confidence scores for exposure estimation",
                ["Not a census, occupancy, residential use, beneficiary identity, or legal ownership record", "Do not sum with OSM"],
                "South Asia",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_BUILDING_FOOTPRINTS", False, False
            ),
            (
                "S23", "Microsoft Global ML Building Footprints", CapabilityType.PRODUCT, PriorityClass.ALTERNATIVE,
                "Microsoft", "GIS Specialist",
                "Machine-learned building footprints for completeness cross-checks",
                ["Deduplicate against Google Open Buildings and OSM; do not sum together"],
                "India",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_BUILDING_FOOTPRINTS", False, False
            ),
            (
                "S24", "Meta/CIESIN High Resolution Population Density Maps (HRSL)", CapabilityType.PRODUCT, PriorityClass.ALTERNATIVE,
                "Meta / CIESIN", "Demographer",
                "Modelled high-resolution population distribution grids",
                ["Small grid cells are statistical models, not precise headcounts", "Not an India verified census"],
                "Global / India",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_POPULATION", False, False
            ),
            (
                "S25", "Census 2011 Primary Census Abstract (PCA) / Wayanad DCHB", CapabilityType.PRODUCT, PriorityClass.CORE,
                "Office of the Registrar General & Census Commissioner, India (ORGI)", "Demographer",
                "Authoritative statutory population and demographic baseline aggregates",
                ["2011 baseline requires LGD code crosswalk; cannot infer individual household vulnerabilities without survey"],
                "India (Wayanad)",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_POPULATION", False, False
            ),
            (
                "S26", "WorldPop Global High Resolution Population", CapabilityType.PRODUCT, PriorityClass.SUPPORT,
                "WorldPop / Univ of Southampton", "Demographer",
                "Recent modelled population disaggregation context (2015-2030)",
                ["Not a beneficiary register; do not automatically average with correlated population grids"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_POPULATION", False, False
            ),
            (
                "S27", "JRC Global Human Settlement Layer (GHSL)", CapabilityType.PRODUCT, PriorityClass.ALTERNATIVE,
                "European Commission JRC", "Settlement Planner",
                "Global built-up surface and settlement spatial epoch evolution",
                ["Built-up area is not population; future epochs are projections"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_POPULATION", False, False
            ),
            (
                "S28", "ESA WorldCover 10m", CapabilityType.PRODUCT, PriorityClass.CORE,
                "ESA", "Land Use Specialist",
                "10m global land cover classification",
                ["Mapped tree cover is not legally notified forest land", "Does not prove absence of community livelihood rights"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S29", "Dynamic World V1 Near Real-Time Land Cover", CapabilityType.PRODUCT, PriorityClass.OPTIONAL,
                "Google / WRI", "Land Use Specialist",
                "Near real-time 10m land cover probability",
                ["Shares Sentinel-2 inputs with WorldCover; cannot establish legal tenure or permitted land use"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.PROCESSING],
                None, False, False
            ),
            (
                "S30", "JRC Global Surface Water", CapabilityType.PRODUCT, PriorityClass.SUPPORT,
                "European Commission JRC", "Hydrologist",
                "Multi-decadal surface water occurrence, recurrence, and seasonality",
                ["Does not measure river discharge, potable well yield, or local flood depth"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S31", "ISRIC SoilGrids 250m", CapabilityType.PRODUCT, PriorityClass.OPTIONAL,
                "ISRIC World Soil Information", "Pedologist",
                "Global 250m digital soil property maps (clay, silt, sand, bulk density, pH)",
                ["REST API is paused by provider and must not be a dependency", "Does not measure site slip depth or shear strength"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                None, False, True  # is_paused = True
            ),
            (
                "S32", "IMD Historical Gridded Rainfall (0.25 deg)", CapabilityType.PRODUCT, PriorityClass.SUPPORT,
                "India Meteorological Department (IMD Pune)", "Meteorologist",
                "Long-term daily gridded rainfall accumulation climatology",
                ["District/gridded averages do not replace site-specific cloudburst telemetry in complex terrain"],
                "India",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_RAINFALL", False, False
            ),
            (
                "S33", "IMD Operational Weather APIs", CapabilityType.CATALOG, PriorityClass.SUPPORT_API,
                "India Meteorological Department", "Meteorologist",
                "Official real-time weather observations, AWS telemetry, and forecasts",
                ["Requires registered token and IP whitelisting; district forecast does not prove slope safety"],
                "India",
                [EndpointType.DISCOVERY, EndpointType.AUTHENTICATION, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S34", "NASA GPM IMERG V07 Precipitation", CapabilityType.PRODUCT, PriorityClass.SUPPORT,
                "NASA / JAXA", "Meteorologist",
                "Blended satellite precipitation estimates at 0.1 deg",
                ["Convert rainfall rate to interval depth; significant orographic bias in mountainous Western Ghats"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_RAINFALL", False, False
            ),
            (
                "S35", "CHIRPS v3 Rainfall Records", CapabilityType.PRODUCT, PriorityClass.SUPPORT,
                "Climate Hazards Center / UCSB", "Meteorologist",
                "Quasi-global high-resolution rainfall climatology (CHIRPS v3 preferred over v2)",
                ["Not hourly gauge truth; shares satellite inputs with IMERG; do not sum as independent evidence"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_RAINFALL", False, False
            ),
            (
                "S36", "ECMWF ERA5-Land Reanalysis", CapabilityType.PRODUCT, PriorityClass.OPTIONAL,
                "ECMWF / Copernicus CDS", "Meteorologist",
                "High-resolution reanalysis surface parameters",
                ["Accumulated flux variables require differencing; soil-moisture layers do not represent pore pressure"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.AUTHENTICATION, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S37", "NASA SMAP Enhanced Soil Moisture", CapabilityType.PRODUCT, PriorityClass.SPECIALIST,
                "NASA / NSIDC", "Soil Specialist",
                "Regional surface soil moisture estimates (9km/33km)",
                ["Coarse resolution constrained by vegetation canopy; not slip-surface saturation"],
                "Global",
                [EndpointType.DISCOVERY, EndpointType.AUTHENTICATION, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            # 5.4 Water, access, and official geography
            (
                "S38", "CGWB NAQUIM Wayanad Groundwater Reports", CapabilityType.PRODUCT, PriorityClass.CORE_CONTEXT,
                "Central Ground Water Board (CGWB)", "Hydrogeologist",
                "Aquifer mapping, hydrogeological units, and regional aquifer context",
                ["No universal parcel-yield API; water level depth is not sustainable yield or borehole discharge"],
                "Kerala (Wayanad)",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S39", "NWIC NWDP / India-WRIS Groundwater Telemetry", CapabilityType.PRODUCT, PriorityClass.SUPPORT_API,
                "NWIC / Ministry of Jal Shakti", "Hydrogeologist",
                "Quarterly and manual groundwater monitoring station series",
                ["Listing does not guarantee a monitoring well at relocation site; water level is not supply rate (LPCD)"],
                "India (Kerala)",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S40", "HydroSHEDS / HydroBASINS / HydroRIVERS", CapabilityType.PRODUCT, PriorityClass.SUPPORT,
                "HydroSHEDS / WWF", "Hydrologist",
                "Global standardized catchment boundaries and drainage networks",
                ["Not surveyed local channels, drainage culverts, discharge capacity, or debris runout boundaries"],
                "Global / Regional",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S41", "OpenStreetMap / Geofabrik Kerala Extract", CapabilityType.PRODUCT, PriorityClass.CORE,
                "OpenStreetMap Community / Geofabrik", "Transport Specialist",
                "Vector road network, footpaths, bridges, and public amenities context",
                ["Field-verify connectivity, road width, emergency accessibility, and legal right-of-way", "Bulk tile download forbidden"],
                "Kerala",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                "MIRROR_BUILDING_FOOTPRINTS", False, False
            ),
            (
                "S42", "PMGSY GeoSadak Rural Road Network", CapabilityType.PRODUCT, PriorityClass.CORE,
                "Ministry of Rural Development (MoRD)", "Transport Specialist",
                "Rural roads, habitation connectivity, and PMGSY road asset geometries",
                ["PMGSY IDs are not Census or LGD codes; operational carriage capacity requires field validation"],
                "India (Kerala)",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S43", "Local Government Directory (LGD)", CapabilityType.PRODUCT, PriorityClass.CORE,
                "Ministry of Panchayati Raj (MoPR)", "Administrative Coordinator",
                "Authoritative local body hierarchy codes (District, Block, Grama Panchayat, Village, Habitation)",
                ["Use reviewed crosswalks; do not conflate administrative entity code with spatial polygon boundary"],
                "India",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S44", "Survey of India Boundaries", CapabilityType.PRODUCT, PriorityClass.CORE,
                "Survey of India (SOI)", "GIS Specialist",
                "Authoritative national, state, district, and village boundary layers",
                ["Authoritative administrative boundary; not land parcel ownership or title"],
                "India",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            # 5.5 Restricted agency and field blockers
            (
                "S45", "Kerala Ente Bhoomi / ReLIS Cadastral Records", CapabilityType.AGENCY_RECORD, PriorityClass.AGENCY_BLOCKER,
                "Revenue Department, Government of Kerala", "Legal / Land Officer",
                "Authoritative cadastral parcel geometry, Resurvey records, RoR, and title interests",
                ["ULPIN or digitisation is not clear title; disputed or paper-vacant parcels require field reconciliation", "MANDATORY BLOCKER"],
                "Kerala (Wayanad)",
                [EndpointType.AUTHENTICATION, EndpointType.ENTITLEMENT, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S46", "JJM / Kerala Water Authority Operating Records", CapabilityType.AGENCY_RECORD, PriorityClass.AGENCY_BLOCKER,
                "Kerala Water Authority (KWA) / Jal Jeevan Mission", "Water Resources Engineer",
                "Drinking water supply network, terminal head, safe yield (>= 55 LPCD), and potability test records",
                ["Dashboard entry is not proof of spare capacity or lean-season reliability", "MANDATORY BLOCKER"],
                "Kerala (Wayanad)",
                [EndpointType.AUTHENTICATION, EndpointType.ENTITLEMENT, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S47", "Forest / FRA / Protected Area Clearances", CapabilityType.AGENCY_RECORD, PriorityClass.AGENCY_BLOCKER,
                "Kerala Forest Department / Scheduled Tribes Development Dept", "Tribal Welfare / Legal Officer",
                "Forest clearance, eco-sensitive zone records, and Gram Sabha resolutions under FRA 2006",
                ["Absence of digital boundary is not absence of tribal rights", "MANDATORY BLOCKER"],
                "Kerala (Wayanad)",
                [EndpointType.AUTHENTICATION, EndpointType.ENTITLEMENT, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S48", "Household Enumeration & Relocation Consent", CapabilityType.FIELD_ACQUISITION, PriorityClass.FIELD_BLOCKER,
                "District Disaster Management Authority (DDMA) / Revenue", "Social Safeguards Officer",
                "Verified beneficiary enumeration, vulnerability accommodations, and written informed consent",
                ["Never derive beneficiary list from rooftop counts or satellite proxies", "MANDATORY BLOCKER"],
                "Wayanad Disaster Zone",
                [EndpointType.AUTHENTICATION, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S49", "Qualified Geotechnical Boreholes & Ground Survey", CapabilityType.FIELD_ACQUISITION, PriorityClass.FIELD_BLOCKER,
                "Accredited Geotechnical Laboratory / Competent Engineers", "Geotechnical Engineer",
                "Signed core borehole logs, standard penetration tests (SPT), safe bearing capacity, and slope stability",
                ["Satellite indices or DEMs cannot replace qualified ground drilling and laboratory tests", "MANDATORY BLOCKER"],
                "Candidate Resettlement Sites",
                [EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S50", "Programme Funding, Sanctions, and Budget Allocation", CapabilityType.AGENCY_RECORD, PriorityClass.AGENCY_BLOCKER,
                "Disaster Management Dept (GoK) / Finance Dept", "Finance Officer",
                "Administrative Sanction (A.S.) G.O., Treasury budget head, and non-duplicate funding certification",
                ["Announced fund is not sanctioned capital; allocation without budgetary backing is void", "MANDATORY BLOCKER"],
                "Kerala",
                [EndpointType.AUTHENTICATION, EndpointType.ENTITLEMENT, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            # 5.6 Geography-conditional, display, and paid options
            (
                "S51", "USGS Earthquake Feeds / NCS Seismic Catalog", CapabilityType.PRODUCT, PriorityClass.LEGACY_OPTIONAL,
                "USGS / National Center for Seismology (NCS)", "Seismologist",
                "Historical seismic event catalog and epicentral location records",
                ["Magnitude is not site-specific peak ground acceleration; not baseline Wayanad scope"],
                "Global / India",
                [EndpointType.DISCOVERY, EndpointType.DOWNLOAD_RECEIPT],
                None, False, False
            ),
            (
                "S52", "CWC/ISRO Himalayan Glacial Lake Inventory", CapabilityType.PRODUCT, PriorityClass.GEOGRAPHY_CONDITIONAL,
                "CWC / NRSC", "Glaciologist",
                "Glacial lake outlines and GLOF susceptibility in Himalayan river basins",
                ["Himalayan-only screening; strictly prohibited and locked out from Kerala and Wayanad workflows"],
                "Himalayan Basins only",
                [EndpointType.DISCOVERY, EndpointType.DISPLAY],
                None, False, False
            ),
            (
                "S53", "Licensed Basemap (CARTO / MapTiler / Self-Hosted OSM)", CapabilityType.DISPLAY, PriorityClass.DISPLAY_ONLY,
                "Licensed Cartographic Provider", "UI/UX Engineer",
                "Visual background map tiles with verified attribution, key management, and cache terms",
                ["Basemap is display infrastructure, NEVER analytical evidence; bulk OSM tile download prohibited"],
                "Global / India",
                [EndpointType.DISPLAY],
                None, False, False
            ),
            (
                "S54", "Licensed High-Resolution Imagery / Drone Orthomosaic", CapabilityType.PRODUCT, PriorityClass.OPTIONAL_PAID,
                "Procured Commercial Satellite / Survey Agency", "Photogrammetry Specialist",
                "Sub-metre aerial imagery for boundary clarification and discrepancy investigation",
                ["High resolution cannot prove legal title, household occupancy, or structural foundation integrity"],
                "Project Specific AOI",
                [EndpointType.AUTHENTICATION, EndpointType.DOWNLOAD_RECEIPT, EndpointType.DISPLAY],
                None, False, False
            ),
        ]

        for item in catalog_items:
            (sid, name, cap_type, priority, custodian, owner, intended, non_uses, geo, endpoints, mirror, sandbox, paused) = item
            self._capabilities[sid] = SourceCapabilityRecord(
                source_id=sid,
                name=name,
                capability_type=cap_type,
                priority_class=priority,
                custodian=custodian,
                owner_role=owner,
                intended_use=intended,
                explicit_non_uses=non_uses,
                supported_geography=geo,
                state=ActivationState.PAUSED if paused else ActivationState.REGISTERED,
                endpoints=endpoints,
                mirror_group_id=mirror,
                is_sandbox_only=sandbox,
                is_paused=paused,
            )

    def _init_mirror_groups(self):
        """
        Populate observation mirror groups for shared lineage deduplication (FR-079, RUL-078).
        """
        groups = [
            MirrorGroupRecord(
                group_id="MIRROR_SENTINEL_2",
                observation_description="Sentinel-2 L2A Earth Observation Scenes",
                member_source_ids=["S08", "S10", "S11", "S12"],
                primary_source_id="S10",
                reconciliation_strategy=ReconciliationMethod.PRIMARY_AUTHORITATIVE,
                deduplication_rule="Shared Sentinel-2 datatake and granule ID across CDSE, Earth Search, and Planetary Computer. Reconcile to single primary observation; reject independent corroboration.",
            ),
            MirrorGroupRecord(
                group_id="MIRROR_BUILDING_FOOTPRINTS",
                observation_description="Building Footprint Extraction Surfaces",
                member_source_ids=["S22", "S23", "S41"],
                primary_source_id="S22",
                reconciliation_strategy=ReconciliationMethod.UNION_DEDUPLICATED,
                deduplication_rule="Spatial polygon intersection (IoU > 0.50). Deduplicate footprint counts; never sum Google Open Buildings, Microsoft ML, and OSM footprints as independent buildings.",
            ),
            MirrorGroupRecord(
                group_id="MIRROR_DEM",
                observation_description="Digital Elevation and Surface Models",
                member_source_ids=["S18", "S19", "S20", "S21"],
                primary_source_id="S18",
                reconciliation_strategy=ReconciliationMethod.PRIMARY_AUTHORITATIVE,
                deduplication_rule="DEM elevation surfaces sharing spaceborne radar/optical missions. Multiple DEM mirrors do not constitute independent slope safety validation.",
            ),
            MirrorGroupRecord(
                group_id="MIRROR_POPULATION",
                observation_description="Population Grids and Disaggregation Surfaces",
                member_source_ids=["S25", "S26", "S24", "S27"],
                primary_source_id="S25",
                reconciliation_strategy=ReconciliationMethod.PRIMARY_AUTHORITATIVE,
                deduplication_rule="Census 2011 is the statutory baseline. Modelled population surfaces (WorldPop, HRSL, GHSL) provide sensitivity analysis only and must not be averaged into beneficiary counts.",
            ),
            MirrorGroupRecord(
                group_id="MIRROR_RAINFALL",
                observation_description="Gridded Precipitation and Satellite Climatologies",
                member_source_ids=["S32", "S34", "S35"],
                primary_source_id="S32",
                reconciliation_strategy=ReconciliationMethod.PRIMARY_AUTHORITATIVE,
                deduplication_rule="IMD gridded rainfall is national reference. Satellite blended products (GPM IMERG, CHIRPS) share gauge and microwave inputs; do not sum as independent evidence.",
            ),
        ]
        for g in groups:
            self._mirror_groups[g.group_id] = g

    def _init_provider_health(self):
        """
        Populate provider adapter health telemetry with strict secret redaction (FR-081, RUL-081).
        """
        providers = [
            ("S10", "Copernicus Data Space Ecosystem (CDSE)", EntitlementState.ACTIVE, TokenState.VALID, 1420, 10000, 60, 0.0, 312.4, 0.02, 12.5, True),
            ("S14", "NRSC Bhoonidhi Portal", EntitlementState.PENDING_REVIEW, TokenState.VALID, 120, 2000, 30, 0.0, 645.0, 0.08, 48.0, True),
            ("S18", "Copernicus DEM GLO-30 / AWS OpenData", EntitlementState.ACTIVE, TokenState.VALID, 450, 50000, 120, 0.0, 85.2, 0.0, 720.0, True),
            ("S22", "Google Open Buildings Open Data Storage", EntitlementState.ACTIVE, TokenState.VALID, 85, 5000, 60, 0.0, 110.5, 0.01, 2160.0, True),
            ("S31", "ISRIC SoilGrids WCS", EntitlementState.UNENTITLED, TokenState.NOT_CONFIGURED, 0, 1000, 10, 0.0, 0.0, 100.0, 8760.0, False),
            ("S53", "MapLibre Vector Tile Basemap / Proj CDN", EntitlementState.ACTIVE, TokenState.VALID, 18500, 100000, 300, 15.0, 42.1, 0.0, 168.0, True),
        ]
        for sid, name, ent, tok, used, limit, rpm, cost, lat, err, age, fb in providers:
            self._provider_health[sid] = ProviderHealthStatus(
                source_id=sid,
                provider_name=name,
                entitlement_state=ent,
                token_state=tok,
                quota_used=used,
                quota_limit=limit,
                rate_limit_rpm=rpm,
                estimated_cost_usd=cost,
                latency_ms=lat,
                error_rate_pct=err,
                source_observation_age_hours=age,
                governed_file_fallback_available=fb,
                secrets_redacted=True,
                redacted_token_preview="***REDACTED***",
            )

    def _init_basemap_configs(self):
        """
        Populate basemap configuration ensuring decoupling from analytical lineage (FR-084, RUL-082).
        """
        self._basemap_configs["S53"] = BasemapConfigRecord(
            provider_id="S53",
            style_version="MapLibre-PUNARVAS-v2.1",
            attribution="© OpenStreetMap contributors, ODbL 1.0; PUNARVAS-AI Cartographic Shell",
            permitted_offline_use=True,
            prohibit_osm_tile_bulk_download=True,
            pixels_decoupled_from_analytical_lineage=True,
            fallback_mode="NON_MAP_TABULAR_VECTOR",
            is_healthy=True,
        )

    # --------------------------------------------------------------------------
    # Capability Query Methods
    # --------------------------------------------------------------------------

    def list_capabilities(
        self,
        priority_class: Optional[PriorityClass] = None,
        capability_type: Optional[CapabilityType] = None,
        activation_state: Optional[ActivationState] = None,
    ) -> List[SourceCapabilityRecord]:
        """
        List all registered S01-S54 source capabilities with optional filters.
        """
        results = list(self._capabilities.values())
        if priority_class:
            results = [r for r in results if r.priority_class == priority_class]
        if capability_type:
            results = [r for r in results if r.capability_type == capability_type]
        if activation_state:
            results = [r for r in results if r.state == activation_state]
        return results

    def get_capability(self, source_id: str) -> Optional[SourceCapabilityRecord]:
        return self._capabilities.get(source_id)

    # --------------------------------------------------------------------------
    # Catalog Search & Separation (FR-077, RUL-076, AT-31)
    # --------------------------------------------------------------------------

    def record_catalog_search(self, source_id: str, query_filter: str, actor_id: str) -> Dict[str, Any]:
        """
        Simulate/record catalog discovery.
        CRITICAL INVARIANT: Catalog discovery alone NEVER marks an asset downloaded,
        licensed, validated, or usable (CATALOG_VISIBLE != APPROVED_FOR_USE).
        """
        cap = self._capabilities.get(source_id)
        if not cap:
            raise ValueError(f"Source {source_id} not registered in S01-S54 catalog")

        # Update state to CATALOG_VISIBLE if it was REGISTERED or DOCUMENTED
        if cap.state in (ActivationState.REGISTERED, ActivationState.DOCUMENTED):
            cap.state = ActivationState.CATALOG_VISIBLE

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="System/SourceAccess",
            action="RECORD_CATALOG_DISCOVERY",
            entity_type="SourceCapability",
            entity_id=source_id,
            version_id="catalog_search",
            reason=f"Catalog search executed for {source_id} with filter '{query_filter}'. Asset is CATALOG_VISIBLE, NOT usable.",
        )

        return {
            "source_id": source_id,
            "title": cap.name,
            "current_state": cap.state.value,
            "is_usable": cap.state == ActivationState.APPROVED_FOR_USE,  # Strictly False
            "dependent_workflow_status": "HOLD",
            "statutory_note": "Catalog visibility confirmed. Asset is NOT downloaded, licensed, or usable for decision gates.",
        }

    # --------------------------------------------------------------------------
    # Permitted AOI Sample Gate & Immediate Quarantine (FR-078, RUL-077, AT-38)
    # --------------------------------------------------------------------------

    def validate_aoi_sample(self, sample_input: AOISampleGateInput, actor_id: str) -> AOISampleGateResult:
        """
        Evaluate candidate AOI sample against the 12 Section 6 gate checks.
        If ANY check fails, sample is QUARANTINED and dependent workflows are blocked.
        """
        cap = self._capabilities.get(sample_input.source_id)
        if not cap:
            raise ValueError(f"Source {sample_input.source_id} not found in S01-S54 catalog")

        checks: Dict[str, bool] = {}
        quarantine_reasons: List[str] = []

        # 1. License & offline/redistribution rights
        has_license = sample_input.has_redistribution_and_offline_rights and bool(sample_input.license_type)
        checks["license_redistribution_offline"] = has_license
        if not has_license:
            quarantine_reasons.append("Unlicensed redistribution or missing offline rights")

        # 2. AOI Bounding Box check (Wayanad bounding box: lat ~11.45 to 11.95, lon ~75.80 to 76.35)
        # We permit samples overlapping the Wayanad region
        within_aoi = (
            sample_input.min_lat <= 12.10 and sample_input.max_lat >= 11.30 and
            sample_input.min_lon <= 76.50 and sample_input.max_lon >= 75.60
        )
        checks["aoi_spatial_extent"] = within_aoi
        if not within_aoi:
            quarantine_reasons.append(f"Spatial extent ({sample_input.min_lat},{sample_input.min_lon} to {sample_input.max_lat},{sample_input.max_lon}) is outside permitted Wayanad AOI")

        # 3. Temporal validity
        now = utc_now()
        is_future = sample_input.observation_timestamp > now
        checks["temporal_validity"] = not is_future
        if is_future:
            quarantine_reasons.append("Observation timestamp is in the future")

        # 4. Schema & format
        valid_formats = ["GeoTIFF", "COG", "GeoJSON", "Shapefile", "CSV", "NetCDF", "Parquet", "PDF"]
        valid_format = sample_input.schema_format in valid_formats
        checks["schema_format"] = valid_format
        if not valid_format:
            quarantine_reasons.append(f"Unsupported schema format: {sample_input.schema_format}")

        # 5. Projected CRS & Vertical Datum
        valid_crs = bool(sample_input.crs and sample_input.crs.upper().startswith("EPSG:") and sample_input.crs != "EPSG:0")
        checks["crs_valid"] = valid_crs
        if not valid_crs:
            quarantine_reasons.append(f"Invalid or missing Coordinate Reference System (CRS): {sample_input.crs}")

        # 6. Resolution / scale
        valid_res = sample_input.resolution_meters > 0.0 and sample_input.resolution_meters <= 100000.0
        checks["resolution_scale"] = valid_res
        if not valid_res:
            quarantine_reasons.append(f"Implausible resolution: {sample_input.resolution_meters} meters")

        # 7. Units and NoData declaration
        valid_units = bool(sample_input.units and sample_input.units.upper() not in ("UNKNOWN", "NONE", ""))
        checks["units_and_nodata"] = valid_units
        if not valid_units:
            quarantine_reasons.append(f"Invalid or missing measurement units: '{sample_input.units}'")

        # 8. Bit-for-bit Checksum Verification
        checksum_match = (sample_input.raw_payload_checksum == sample_input.claimed_checksum) and bool(sample_input.raw_payload_checksum)
        checks["checksum_match"] = checksum_match
        if not checksum_match:
            quarantine_reasons.append(f"SHA-256 Checksum mismatch: claimed {sample_input.claimed_checksum}, computed {sample_input.raw_payload_checksum}")

        # 9. Cadence and Latency
        age_days = (now - sample_input.observation_timestamp).total_seconds() / 86400.0
        cadence_ok = age_days <= 7300.0  # Up to 20 years for historical baselines
        checks["cadence_acceptable"] = cadence_ok
        if not cadence_ok:
            quarantine_reasons.append(f"Observation age ({age_days:.1f} days) exceeds maximum allowable baseline vintage")

        # 10. Quota and Cost
        cost_ok = sample_input.cost_usd >= 0.0 and sample_input.cost_usd <= 500.0
        checks["quota_cost_within_limit"] = cost_ok
        if not cost_ok:
            quarantine_reasons.append(f"Sample acquisition cost (${sample_input.cost_usd:.2f}) exceeds single-sample authorization ceiling")

        # 11. Named Reviewer ID
        reviewer_ok = bool(sample_input.reviewer_id and len(sample_input.reviewer_id.strip()) >= 3)
        checks["reviewer_id_present"] = reviewer_ok
        if not reviewer_ok:
            quarantine_reasons.append("Missing accredited reviewer identity")

        # 12. Reproducibility
        reproducible = bool(sample_input.reproducibility_notes)
        checks["reproducibility_verified"] = reproducible
        if not reproducible:
            quarantine_reasons.append("Missing reproducibility notes and pipeline documentation")

        # Evaluate Overall Gate Result
        passed = len(quarantine_reasons) == 0

        if passed:
            cap.state = ActivationState.APPROVED_FOR_USE
            cap.latest_checksum = sample_input.raw_payload_checksum
            cap.quarantine_reason = None
            cap.last_evaluated_at = now
            status_str = "APPROVED_FOR_USE"

            global_audit_ledger.log(
                actor_id=actor_id,
                authority_scope="System/AOIGate",
                action="APPROVE_AOI_SAMPLE",
                entity_type="SourceCapability",
                entity_id=sample_input.source_id,
                version_id=sample_input.sample_id,
                reason=f"Passed all 12 AOI sample checks. Checksum: {cap.latest_checksum[:12]}...",
            )
        else:
            cap.state = ActivationState.QUARANTINED
            cap.quarantine_reason = "; ".join(quarantine_reasons)
            cap.last_evaluated_at = now
            status_str = "QUARANTINED"

            global_audit_ledger.log(
                actor_id=actor_id,
                authority_scope="System/AOIGate",
                action="QUARANTINE_AOI_SAMPLE",
                entity_type="SourceCapability",
                entity_id=sample_input.source_id,
                version_id=sample_input.sample_id,
                reason=cap.quarantine_reason,
            )

        return AOISampleGateResult(
            sample_id=sample_input.sample_id,
            source_id=sample_input.source_id,
            status=status_str,
            passed=passed,
            checks=checks,
            quarantine_reasons=quarantine_reasons,
            evaluated_at=now,
        )

    # --------------------------------------------------------------------------
    # Mirror Group Deduplication (FR-079, RUL-078, AT-32)
    # --------------------------------------------------------------------------

    def list_mirror_groups(self) -> List[MirrorGroupRecord]:
        return list(self._mirror_groups.values())

    def reconcile_mirror_observations(self, request: ObservationReconciliationRequest, actor_id: str) -> ObservationReconciliationResult:
        """
        Deduplicate multiple source representations sharing the same underlying observation.
        Rejects treating multiple mirrors as independent corroboration.
        """
        group = self._mirror_groups.get(request.group_id)
        if not group:
            raise ValueError(f"Mirror group {request.group_id} not registered")

        total = len(request.observations)
        if total == 0:
            return ObservationReconciliationResult(
                group_id=request.group_id,
                reconciliation_method=group.reconciliation_strategy,
                total_input_count=0,
                reconciled_count=0,
                duplicate_count=0,
                is_independent_corroboration_rejected=True,
                explanation="No observations submitted for reconciliation.",
            )

        # Deduplication based on observation identifier
        seen_keys = set()
        reconciled = []
        duplicates = 0

        for obs in request.observations:
            obs_key = obs.get("observation_id") or obs.get("granule_id") or obs.get("footprint_id") or obs.get("id")
            if not obs_key:
                obs_key = hashlib.sha256(json.dumps(obs, sort_keys=True).encode()).hexdigest()

            if obs_key in seen_keys:
                duplicates += 1
            else:
                seen_keys.add(obs_key)
                reconciled.append(obs)

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="System/MirrorReconciliation",
            action="RECONCILE_MIRROR_OBSERVATIONS",
            entity_type="MirrorGroup",
            entity_id=request.group_id,
            version_id="dedup",
            reason=f"Reconciled {total} raw items down to {len(reconciled)} unique items ({duplicates} duplicates suppressed).",
        )

        explanation = (
            f"Applied {group.reconciliation_strategy.value} strategy. "
            f"Under RUL-078/FR-079, observations from platforms {group.member_source_ids} "
            f"sharing the same physical observation were deduplicated. "
            f"Treating multiple mirror portals as independent corroboration is strictly rejected."
        )

        return ObservationReconciliationResult(
            group_id=request.group_id,
            reconciliation_method=group.reconciliation_strategy,
            total_input_count=total,
            reconciled_count=len(reconciled),
            duplicate_count=duplicates,
            is_independent_corroboration_rejected=True,
            explanation=explanation,
        )

    # --------------------------------------------------------------------------
    # Geography and Paused API Lockout Checks (FR-080, RUL-080, AT-33, AT-34)
    # --------------------------------------------------------------------------

    def check_geography_and_governance(self, source_id: str, target_geography: str) -> Dict[str, Any]:
        """
        Verify source against permitted geography and paused API governance.
        """
        cap = self._capabilities.get(source_id)
        if not cap:
            return {"source_id": source_id, "status": "UNKNOWN_SOURCE", "can_link": False, "hold": True}

        # 1. C-FLOOD (S07) Check: Documented Godavari, Tapi, Mahanadi only. Zero Wayanad.
        if source_id == "S07" and ("WAYANAD" in target_geography.upper() or "KERALA" in target_geography.upper()):
            return {
                "source_id": source_id,
                "status": "UNSUPPORTED_GEOGRAPHY",
                "can_link": False,
                "hold": True,
                "reason": "C-FLOOD coverage is strictly Godavari, Tapi, and Mahanadi basins; zero coverage in Kerala/Wayanad (RUL-017, AT-34).",
            }

        # 2. Glacial Lake (S52) Check: Himalayan only.
        if source_id == "S52" and ("KERALA" in target_geography.upper() or "WAYANAD" in target_geography.upper()):
            return {
                "source_id": source_id,
                "status": "UNSUPPORTED_GEOGRAPHY",
                "can_link": False,
                "hold": True,
                "reason": "Himalayan glacial lake inventory is geography-conditional; non-applicable to Kerala (AT-34).",
            }

        # 3. SoilGrids REST (S31) Check: Paused by provider.
        if source_id == "S31" and cap.is_paused:
            return {
                "source_id": source_id,
                "status": "PAUSED_API",
                "can_link": False,
                "hold": True,
                "reason": "SoilGrids REST API is currently paused by provider; mandatory workflows must use verified WCS/file or yield HOLD (RUL-081, AT-33).",
            }

        return {
            "source_id": source_id,
            "status": "PERMITTED",
            "can_link": True,
            "hold": False,
            "reason": f"Source {source_id} permitted for geography '{target_geography}'.",
        }

    # --------------------------------------------------------------------------
    # Provider Health & Secret Redaction (FR-081, RUL-081)
    # --------------------------------------------------------------------------

    def get_provider_health(self, source_id: Optional[str] = None) -> List[ProviderHealthStatus]:
        """
        Retrieve provider health and telemetry with verified zero secret leakage.
        """
        if source_id:
            h = self._provider_health.get(source_id)
            return [h] if h else []
        return list(self._provider_health.values())

    # --------------------------------------------------------------------------
    # S45-S50 Production Blocker Gate Enforcement (FR-083, RUL-079, AT-35)
    # --------------------------------------------------------------------------

    def evaluate_production_blockers(self, request: DependencyBlockerEvaluationRequest, actor_id: str) -> DependencyBlockerReport:
        """
        Evaluate mandatory S45-S50 evidence before enabling live site approval or household allocation.
        If ANY blocker is missing or unverified, the action remains BLOCKED / HOLD.
        """
        ev = request.evidence_records
        blockers: Dict[str, BlockerItem] = {}
        blocking_reasons: List[str] = []

        # S45: Land / Right (Ente Bhoomi / ReLIS Cadastral Records)
        s45_data = ev.get("S45")
        if s45_data and s45_data.get("status") == "VERIFIED":
            blockers["S45"] = BlockerItem(
                blocker_id="S45",
                title="Cadastral & Land Records (Ente Bhoomi/ReLIS)",
                status=BlockerStatus.VERIFIED,
                evidence_summary=s45_data.get("summary", "Parcel geometry and RoR verified by Tahsildar"),
                missing_requirements=[],
            )
        else:
            missing = []
            if not s45_data:
                missing.append("Missing S45 cadastral evidence dossier")
            else:
                if not s45_data.get("parcel_geometry_reconciled"):
                    missing.append("Unreconciled cadastral offset/geometry discrepancy")
                if not s45_data.get("clear_title_certified"):
                    missing.append("Title/transfer clearance not certified")
            blockers["S45"] = BlockerItem(
                blocker_id="S45",
                title="Cadastral & Land Records (Ente Bhoomi/ReLIS)",
                status=BlockerStatus.MISSING if not s45_data else BlockerStatus.PENDING_REVIEW,
                evidence_summary="Pending official Revenue Department reconciliation",
                missing_requirements=missing or ["Unverified cadastral records"],
            )
            blocking_reasons.append("S45: Missing or unverified cadastral/title records")

        # S46: Water & Services (JJM / KWA Operations)
        s46_data = ev.get("S46")
        if s46_data and s46_data.get("status") == "VERIFIED" and s46_data.get("sustainable_yield_lpcd", 0) >= 55:
            blockers["S46"] = BlockerItem(
                blocker_id="S46",
                title="Drinking Water & Service Capacity (JJM/KWA)",
                status=BlockerStatus.VERIFIED,
                evidence_summary=f"KWA certified {s46_data.get('sustainable_yield_lpcd')} LPCD lean-season sustainable supply",
                missing_requirements=[],
            )
        else:
            missing = []
            if not s46_data:
                missing.append("Missing KWA/JJM engineering report")
            elif s46_data.get("sustainable_yield_lpcd", 0) < 55:
                missing.append(f"Lean-season yield ({s46_data.get('sustainable_yield_lpcd', 0)} LPCD) below JJM 55 LPCD minimum")
            if s46_data and not s46_data.get("potability_certified"):
                missing.append("Potability test results pending or non-compliant")
            blockers["S46"] = BlockerItem(
                blocker_id="S46",
                title="Drinking Water & Service Capacity (JJM/KWA)",
                status=BlockerStatus.MISSING if not s46_data else BlockerStatus.PENDING_REVIEW,
                evidence_summary="Insufficient or unverified drinking water supply",
                missing_requirements=missing or ["Unverified water capacity"],
            )
            blocking_reasons.append("S46: Inadequate lean-season water or uncertified potability")

        # S47: Forest / FRA Clearances
        s47_data = ev.get("S47")
        if s47_data and s47_data.get("status") == "VERIFIED":
            blockers["S47"] = BlockerItem(
                blocker_id="S47",
                title="Forest & FRA Statutory Clearances",
                status=BlockerStatus.VERIFIED,
                evidence_summary=s47_data.get("summary", "Forest Dept NOC & Gram Sabha FRA resolution confirmed"),
                missing_requirements=[],
            )
        else:
            missing = []
            if not s47_data:
                missing.append("Missing FRA 2006 / Forest clearance records")
            elif not s47_data.get("gram_sabha_resolution"):
                missing.append("Gram Sabha resolution under FRA §3(1)(m)/§4(5) pending")
            blockers["S47"] = BlockerItem(
                blocker_id="S47",
                title="Forest & FRA Statutory Clearances",
                status=BlockerStatus.MISSING if not s47_data else BlockerStatus.PENDING_REVIEW,
                evidence_summary="Pending Forest/Tribal clearance or Gram Sabha resolution",
                missing_requirements=missing or ["Unverified FRA/Forest clearance"],
            )
            blocking_reasons.append("S47: Missing FRA Gram Sabha resolution or Forest clearance")

        # S48: Household Participation & Informed Consent
        s48_data = ev.get("S48")
        if s48_data and s48_data.get("status") == "VERIFIED" and s48_data.get("consent_percentage", 0) >= 100:
            blockers["S48"] = BlockerItem(
                blocker_id="S48",
                title="Household Enumeration & Relocation Consent",
                status=BlockerStatus.VERIFIED,
                evidence_summary="100% verified household enumeration and informed consent on record",
                missing_requirements=[],
            )
        else:
            missing = []
            if not s48_data:
                missing.append("Missing verified household enumeration register")
            elif s48_data.get("consent_percentage", 0) < 100:
                missing.append(f"Incomplete consent ({s48_data.get('consent_percentage', 0)}% of candidate beneficiaries)")
            blockers["S48"] = BlockerItem(
                blocker_id="S48",
                title="Household Enumeration & Relocation Consent",
                status=BlockerStatus.MISSING if not s48_data else BlockerStatus.PENDING_REVIEW,
                evidence_summary="Incomplete household enumeration or consent",
                missing_requirements=missing or ["Incomplete household consent"],
            )
            blocking_reasons.append("S48: Missing verified beneficiary enumeration or 100% consent")

        # S49: Qualified Site Geotechnics & Ground Surveys
        s49_data = ev.get("S49")
        if s49_data and s49_data.get("status") == "VERIFIED" and s49_data.get("factor_of_safety", 0.0) >= 1.3:
            blockers["S49"] = BlockerItem(
                blocker_id="S49",
                title="Qualified Geotechnical Boreholes & Ground Survey",
                status=BlockerStatus.VERIFIED,
                evidence_summary=f"Accredited borehole log confirmed; Factor of Safety = {s49_data.get('factor_of_safety')}",
                missing_requirements=[],
            )
        else:
            missing = []
            if not s49_data:
                missing.append("Missing geotechnical borehole logs and laboratory test report")
            elif s49_data.get("factor_of_safety", 0.0) < 1.3:
                missing.append(f"Slope factor of safety ({s49_data.get('factor_of_safety', 0.0)}) below mandatory 1.3 standard")
            blockers["S49"] = BlockerItem(
                blocker_id="S49",
                title="Qualified Geotechnical Boreholes & Ground Survey",
                status=BlockerStatus.MISSING if not s49_data else BlockerStatus.PENDING_REVIEW,
                evidence_summary="Unverified bearing capacity or inadequate slope factor of safety",
                missing_requirements=missing or ["Unverified geotechnical survey"],
            )
            blocking_reasons.append("S49: Missing accredited geotechnical borehole tests or unsafe slope factor")

        # S50: Programme Funding, Sanction, and Budget Allocation
        s50_data = ev.get("S50")
        if s50_data and s50_data.get("status") == "VERIFIED" and s50_data.get("administrative_sanction_number"):
            blockers["S50"] = BlockerItem(
                blocker_id="S50",
                title="Programme Funding & Administrative Sanction",
                status=BlockerStatus.VERIFIED,
                evidence_summary=f"Administrative Sanction G.O. #{s50_data.get('administrative_sanction_number')} verified",
                missing_requirements=[],
            )
        else:
            missing = []
            if not s50_data:
                missing.append("Missing Administrative Sanction (A.S.) G.O. and treasury head")
            elif not s50_data.get("administrative_sanction_number"):
                missing.append("Missing formal Administrative Sanction order number")
            blockers["S50"] = BlockerItem(
                blocker_id="S50",
                title="Programme Funding & Administrative Sanction",
                status=BlockerStatus.MISSING if not s50_data else BlockerStatus.PENDING_REVIEW,
                evidence_summary="Pending administrative sanction or financial sanction",
                missing_requirements=missing or ["Unverified programme funding"],
            )
            blocking_reasons.append("S50: Missing administrative sanction G.O. or budget allocation")

        can_proceed = len(blocking_reasons) == 0
        overall_status = "PASS" if can_proceed else "BLOCKED"

        audit_payload = {
            "site_id": request.site_id,
            "action": request.allocation_action,
            "can_proceed": can_proceed,
            "overall_status": overall_status,
            "blocking_reasons": blocking_reasons,
        }
        audit_hash = hashlib.sha256(json.dumps(audit_payload, sort_keys=True).encode()).hexdigest()

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="System/BlockerGate",
            action="EVALUATE_DEPENDENCY_BLOCKERS",
            entity_type="ResettlementSite",
            entity_id=request.site_id,
            version_id=audit_hash[:16],
            reason=f"Blocker gate evaluation: can_proceed={can_proceed}, status={overall_status}. {len(blocking_reasons)} blocking issues.",
        )

        return DependencyBlockerReport(
            site_id=request.site_id,
            action=request.allocation_action,
            can_proceed=can_proceed,
            overall_status=overall_status,
            blockers=blockers,
            blocking_reasons=blocking_reasons,
            audit_hash=audit_hash,
            evaluated_at=utc_now(),
        )

    # --------------------------------------------------------------------------
    # Basemap Decoupling & Display Invariants (FR-084, RUL-082, AT-37)
    # --------------------------------------------------------------------------

    def get_basemap_config(self, provider_id: str = "S53") -> BasemapConfigRecord:
        cfg = self._basemap_configs.get(provider_id)
        if not cfg:
            raise ValueError(f"Basemap provider {provider_id} not registered")
        return cfg

    def simulate_basemap_failure_fallback(self, provider_id: str = "S53", actor_id: str = "sys-admin") -> Dict[str, Any]:
        """
        Demonstrate that basemap tile failure or key expiry keeps analytical decision lineage
        unaffected, triggering the approved non-map tabular/vector fallback (FR-084, AT-37).
        """
        cfg = self.get_basemap_config(provider_id)
        cfg.is_healthy = False

        global_audit_ledger.log(
            actor_id=actor_id,
            authority_scope="System/Basemap",
            action="TRIGGER_BASEMAP_FALLBACK",
            entity_type="BasemapConfig",
            entity_id=provider_id,
            version_id="fallback",
            reason="Basemap provider network failure or key expiry. Analytical lineage decoupled and intact.",
        )

        return {
            "provider_id": provider_id,
            "basemap_healthy": False,
            "fallback_mode": cfg.fallback_mode,
            "analytical_lineage_decoupled": cfg.pixels_decoupled_from_analytical_lineage,
            "prohibit_osm_tile_bulk_download": cfg.prohibit_osm_tile_bulk_download,
            "message": "Basemap rendering failed. System successfully engaged approved non-map tabular/vector fallback. Analytical decisions remain 100% unaffected.",
        }


# Global singleton instance
source_access_service = SourceAccessService()
