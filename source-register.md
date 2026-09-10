---
title: Sthira Evidence and Source Register
document_id: PUN-SOURCES
version: 1.2
status: Dated baseline; operational use requires source-version review
as_of: 2026-09-08
audience: Programme, legal, GIS, data, domain, engineering, assurance, and audit teams
owner: Evidence governance lead
normative_scope: Evidence provenance, verified use, coverage, limitations, and unresolved verification
---

# Sthira Evidence and Source Register

## 1. Reading rule

This is a dated evidence register, not a claim that every linked document was fully accessible or that a source approves a product formula. Before operational use, capture the exact retrieved artifact, access date, license/use basis and checksum as a `dataset_version`. Law, scheme rules, beneficiary figures and coverage are effective-dated.

Status meanings: `PRIMARY-VERIFIED` means the identified official source supports the narrow statement recorded here; `PRIMARY-LOCATED` means the official publication exists but the exact operative artifact must be captured/reviewed; `REFERENCE` supports engineering interpretation; `UNRESOLVED` must not drive an official result.

## 2. Legal, programme, and government sources

| ID | Source | Status | Supported use | Limits and required follow-up |
| --- | --- | --- | --- | --- |
| SRC-001 | [Disaster Management (Amendment) Act, 2025](https://prsindia.org/files/bills_acts/acts_parliament/2025/The_Disaster_Management_Act%2C_2025.pdf), §17 amending principal Act §31(4) | PRIMARY-VERIFIED | District Plan review/update at least once every two years or earlier as necessary | Does not set every internal plan/SLA cadence; review all other relied-on sections in consolidated current law |
| SRC-002 | [PIB commencement notice](https://www.pib.gov.in/PressReleasePage.aspx?PRID=2146781) | PRIMARY-VERIFIED | 2025 amendment commenced 9 April 2025 | Capture Gazette notification in legal register before live use |
| SRC-003 | [Disaster Management Act, 2005](https://www.mha.gov.in/sites/default/files/2022-09/The%20Disaster%20Management%20Act,%202005%5B1%5D.pdf) | PRIMARY-LOCATED | Base statutory roles/process context | Read with amendments; do not rely on superseded wording |
| SRC-004 | [RFCTLARR Act](https://www.indiacode.nic.in/bitstream/123456789/2121/1/A2013-30.pdf) and [DoLR rules/resources](https://dolr.gov.in/en/document-category/acts-rules/) | PRIMARY-LOCATED | Acquisition/compensation workflow basis | Kerala rules, exemptions, negotiated routes and current orders require case-specific legal review; no universal duration/multiplier |
| SRC-005 | [Forest Rights Act and Rules](https://tribal.nic.in/FRA/data/FRARulesBook.pdf) | PRIMARY-LOCATED | Individual/community forest-rights process and anti-eviction safeguard | FRA/PESA/Gram Sabha application depends on location, right and action; no blanket formula |
| SRC-006 | [DILRMP 3.0 Operational Guidelines 2026–2031](https://dolr.gov.in/en/document/digital-india-land-records-modernization-programmedilrmp-3-0-operational-guidelines-2026-2031/) | PRIMARY-VERIFIED FOR EXISTENCE | Current land-record modernization context | ULPIN/digitization does not prove title; capture exact downloadable guideline before implementing conformance |
| SRC-007 | [Kerala Vulnerability Linked Relocation Scheme](https://sdma.kerala.gov.in/vulnerability-linked-relocation-scheme/) | PRIMARY-LOCATED | Scheme-specific voluntary/self-relocation precedent | Eligibility, hazards, tenure categories, caps, milestones and effective order must be versioned; not universal assistance |
| SRC-008 | [MHA response-fund material](https://www.ndmindia.mha.gov.in/ndmi/responsefund) | PRIMARY-LOCATED | Evidence that erosion-related resettlement/mitigation funding can exist | Does not establish present household eligibility or current funding; verify funding period/programme |
| SRC-009 | [Kerala LSG DM Plans](https://sdma.kerala.gov.in/local-self-government-dm-plans/) | PRIMARY-VERIFIED | Participatory plan/template integration | Product produces evidence/annexes; it does not replace consultation or approval |
| SRC-010 | [KSDMA Wayanad reports/PDNA](https://sdma.kerala.gov.in/reports-landslides-2024/) and [orders](https://sdma.kerala.gov.in/government-orders-5/) | PRIMARY-LOCATED | Dated Wayanad facts and workflow evidence | Counts, sites, beneficiaries, costs and progress are versions, never constants |

## 3. Hazard, mapping, population, and service sources

| ID | Source | Status | Supported use | Limits and required follow-up |
| --- | --- | --- | --- | --- |
| SRC-011 | [KSDMA hazard maps](https://sdma.kerala.gov.in/hazard-maps/) | PRIMARY-VERIFIED | Kerala/GSI hazard-product discovery, including Wayanad | Capture exact layer, scale, vintage, license, NoData and supersession; susceptibility is not notification or forecast |
| SRC-012 | [C-FLOOD official release](https://www.pib.gov.in/PressReleasePage.aspx?PRID=2149301) | PRIMARY-VERIFIED FOR INITIAL COVERAGE | Documents initial Godavari, Tapi and Mahanadi coverage | Does not establish Wayanad coverage; re-check current documented coverage before any new use |
| SRC-013 | [Jal Jeevan Mission](https://jaljeevanmission.gov.in/about_jjm) | PRIMARY-VERIFIED | Applicable rural domestic service baseline of 55 LPCD | Not proof of sustainable yield, quality, rights, other demand, storage or delivery |
| SRC-014 | [Census Primary Census Abstract](https://censusindia.gov.in/nada/index.php/catalog/6191) | PRIMARY-LOCATED | Population baseline with vintage | Usually 2011 baseline; aggregation/disaggregation uncertainty must be explicit |
| SRC-015 | [Google Open Buildings](https://sites.research.google/gr/open-buildings/) and [catalog](https://developers.google.com/earth-engine/datasets/catalog/GOOGLE_Research_open-buildings_v3_polygons) | REFERENCE | Building-footprint evidence and exposure estimation | Does not prove dwelling use, occupation, household identity, ownership or complete coverage |
| SRC-016 | [Sentinel-2 catalog](https://developers.google.com/earth-engine/datasets/catalog/COPERNICUS_S2_SR_HARMONIZED) | REFERENCE | Optional remote-sensing discrepancy methods | B8 is 10 m and B11 is 20 m; masking/alignment required; resampling creates no detail |
| SRC-017 | [UNDRR disaster-risk terminology](https://www.undrr.org/terminology/disaster-risk) | REFERENCE | Conceptual risk terminology | Does not authorize `H×E×V/C`, `Ω`, weights, thresholds or legal action |
| SRC-018 | [MoHUA urban open-space clarification](https://www.pib.gov.in/PressReleasePage.aspx?PRID=1813182) | PRIMARY-VERIFIED | 10–12 m²/person relates to recommended urban open space | Must not be used as total settlement land or grazing capacity |

## 4. Privacy, security, accessibility, and interoperability

| ID | Source | Status | Supported use | Limits and required follow-up |
| --- | --- | --- | --- | --- |
| SRC-019 | [DPDP Rules, 2025](https://www.meity.gov.in/documents/act-and-policies/digital-personal-data-protection-rules-2025-gDOxUjMtQWa) | PRIMARY-LOCATED | Effective-dated privacy compliance register | Apply provisions by commencement date; do not assert universal India-hosting from DPDP alone |
| SRC-020 | [CERT-In Directions, 28 April 2022](https://www.cert-in.org.in/PDF/CERT-In_Directions_70B_28.04.2022.pdf) | PRIMARY-VERIFIED | Applicable six-hour incident reporting, contact, synchronized clocks and 180-day India-maintained logs | Confirm entity/system applicability and current directions in legal register |
| SRC-021 | [Indian Geospatial Guidelines, 2021](https://dst.gov.in/sites/default/files/Final%20Approved%20Guidelines%20on%20Geospatial%20Data.pdf) | PRIMARY-LOCATED | Spatial accuracy/ownership/access and publication review | Do not conflate with a blanket personal-data localization rule |
| SRC-022 | [GIGW 3.0](https://guidelines.india.gov.in/gigw3/) | PRIMARY-LOCATED | Government-web quality/accessibility baseline | GIGW references WCAG 2.1 AA; this project separately chooses WCAG 2.2 AA as a higher target |
| SRC-023 | [WCAG 2.2](https://www.w3.org/TR/WCAG22/) | PRIMARY-VERIFIED | WCAG 2.2 AA acceptance target | Manual as well as automated testing required; applies to non-map, bilingual and document flows |
| SRC-024 | [PostgreSQL row security](https://www.postgresql.org/docs/current/ddl-rowsecurity.html) and [view privileges](https://www.postgresql.org/docs/current/rules-privileges.html) | REFERENCE | RLS owner/BYPASSRLS/view risk design | Must test pooled context and every delivery path; RLS is not the only authorization layer |
| SRC-025 | [STAC API Community Standard 1.0](https://docs.ogc.org/cs/25-005/25-005.html) | REFERENCE | Pinned catalog interface candidate | Exact conformance classes remain an open architecture decision |
| SRC-026 | [OGC API Features](https://www.ogc.org/standards/ogcapi-features/) | REFERENCE | Vector interchange | Pin collection/conformance profile per release |
| SRC-027 | [SciPy MILP](https://docs.scipy.org/doc/scipy/reference/generated/scipy.optimize.milp.html) | REFERENCE | Floating-point solver semantics and diagnostics | Solver/package not yet selected; independent exact/domain feasibility validation remains required |
| SRC-028 | [Celery task guidance](https://docs.celeryq.dev/en/stable/userguide/tasks.html) | REFERENCE | Retry/idempotence design | Does not close database-commit/message-publish gap; use transactional outbox/equivalent |
| SRC-029 | [Browser storage eviction](https://developer.mozilla.org/en-US/docs/Web/API/Storage_API/Storage_quotas_and_eviction_criteria) | REFERENCE | Offline loss/eviction threat model | Persistence is not guaranteed; provide recovery and user-visible unsynced state |

## 5. Operational data-source and acquisition register

The supplied `SOURCE_MATRIX.csv`, `SOURCE_MATRIX.json`, and the `Sources` worksheet contain the same 54 records. Both supplied XLSX copies are byte-identical (SHA-256 `b7b5bee4aea9fa602bbb2214e01af11e74a667bcdd88e82620a96491070428be`). Their `S01`–`S54` identifiers are preserved below so acquisition evidence can be reconciled back to the handoff. These are a mix of products, catalogs, processing services, agency-record routes, and field-acquisition routes—not 54 independent datasets or connected APIs.

No row below is marked operational merely because documentation or a catalog was located. Before use, one permitted AOI sample must pass the acquisition gate in §6. Mirrors of the same observation are alternate delivery routes, not independent corroboration.

### 5.1 Hazard, geology, and official programme evidence

| ID | Source/product | Class | Acquisition route | Intended use and controlling qualification |
| --- | --- | --- | --- | --- |
| S01 | [KSDMA-hosted GSI landslide susceptibility](https://sdma.kerala.gov.in/hazard-maps/) | CORE | Public page; governed district-shapefile import | Exposure screening only; verify file, CRS, scale, terms, and supersession. Susceptibility is not runout, probability, legal zoning, or parcel safety. |
| S02 | [KSDMA flood probability maps/rasters](https://sdma.kerala.gov.in/hazard-maps/) | CORE | Public listed files; governed import | Screen flood exposure; preserve return period/scenario/model units, resolution, and NoData. It is not a live forecast. |
| S03 | [KSDMA/Wayanad PDNA, orders, plans, and investigations](https://sdma.kerala.gov.in/reports-landslides-2024/) ([orders](https://sdma.kerala.gov.in/government-orders-5/); [district](https://wayanad.gov.in/en/)) | CORE | Public document import; request underlying current official/GIS records | Programme context and dated facts; an index is not proof the underlying artifact was acquired, and historic counts are not current households. |
| S04 | [GSI Bhukosh/geological maps and reports](https://artefacts.data.gov.in/bhukosh/) ([portal](https://bhukosh.gsi.gov.in)) | CORE | Registered search/download cart and applicable terms | Lithology, inventory, and investigations; not measured strength, soil depth, bearing capacity, or a universal parameter API. |
| S05 | [ISRO Bhuvan thematic services/APIs](https://bhuvan.nrsc.gov.in/wiki/index.php/How_to_use_WMS_services) ([API](https://bhuvan-app1.nrsc.gov.in/api); [themes](https://bhuvan-app1.nrsc.gov.in/thematic)) | SUPPORT | WMS/WMTS; enabled downloads; tokenized thematic/proximity APIs | Context layers only. Rendered WMS is not analysis-ready data and Bhuvan is not a cadastral-title API. |
| S06 | [NDEM](https://ndem.nrsc.gov.in/) | AGENCY | Authorized state/district/agency access or governed export | Integrated disaster products where authorized; do not scrape public dashboards or infer bulk/API rights. |
| S07 | [CWC/C-FLOOD](https://pib.gov.in/PressReleasePage.aspx?PRID=2141608) ([coverage evidence](https://www.sansad.in/getFile/loksabhaquestions/annex/185/AU4741_7aBjEI.pdf?source=pqals)) | CONDITIONAL | Official platform/agency exchange; entitlement unverified | Flood outputs only where coverage is documented. Wayanad coverage is unverified; never use it as a landslide model. |

### 5.2 Earth observation, catalogs, and processing routes

| ID | Source/product | Class | Acquisition route | Intended use and controlling qualification |
| --- | --- | --- | --- | --- |
| S08 | [Sentinel-2 L2A](https://browser.stac.dataspace.copernicus.eu/collections/sentinel-2-l2a) | CORE | CDSE, Earth Search, Planetary Computer, or eligible Earth Engine route | Optical/change/index screening with cloud/shadow masking and band alignment; cannot prove small structures, occupation, title, or legal vacancy. |
| S09 | [Sentinel-1 SAR](https://www.earthdata.nasa.gov/data/platforms/space-based-platforms/sentinel-1) ([HyP3](https://hyp3-docs.asf.alaska.edu/products)) | OPTIONAL | CDSE/ASF; qualified SLC workflow for InSAR | GRD/RTC supports backscatter/water context, not displacement. InSAR requires suitable SLC stacks and specialist validation. |
| S10 | [Copernicus Data Space Ecosystem](https://documentation.dataspace.copernicus.eu/APIs/STAC.html) ([OData](https://documentation.dataspace.copernicus.eu/APIs/OData.html); [quotas](https://documentation.dataspace.copernicus.eu/Quotas.html)) | CORE API | Public STAC discovery; authenticated downloads/processing as applicable | Preferred Sentinel route. Keep catalog, download, and processing adapters separate; SciHub ceased operations in 2023. |
| S11 | [Element 84 Earth Search](https://element84.com/earth-search) | ALTERNATIVE API | Public STAC; asset-specific transfer conditions | Alternate cloud-native discovery/COGs. Open catalog access does not establish every asset's transfer rights or availability. |
| S12 | [Microsoft Planetary Computer](https://planetarycomputer.microsoft.com/docs/) | ALTERNATIVE API | Public STAC; expiring SAS-signed asset URLs | Alternate host/catalog; persist stable item IDs, never tokens, and distinguish public service from paid Pro. |
| S13 | [Google Earth Engine](https://earthengine.google.com/noncommercial/) ([tiers](https://developers.google.com/earth-engine/guides/noncommercial_tiers)) | OPTIONAL PROCESSING | Registered Cloud project, authentication, approved use category, quota/cost review | Hosted analysis only if terms fit the actual government/operational use. Restricted household data stays out by default. |
| S14 | [NRSC Bhoonidhi API](https://bhoonidhi.nrsc.gov.in/bhoonidhi-api/index.html) | ALTERNATIVE API | Account credentials → bearer token; documented search/collections/download endpoints | Indian EO route. API documentation is verified; entitlement, AOI stock, pricing, and exact collection access are not. |
| S15 | [USGS Landsat Collection 2/M2M](https://www.usgs.gov/landsat-missions/landsat-data-access) | SUPPORT API | EarthExplorer/ERS and M2M application token; Landsat STAC | Long history for land/water/vegetation; masking and scale/offset required, and 30 m data cannot resolve household occupation. |
| S16 | [NASA ASF Search/HyP3](https://docs.asf.alaska.edu/) | SPECIALIST | ASF discovery; Earthdata Login for downloads/jobs | Specialist SAR processing alternative, not an approved warning or universal displacement product. |
| S17 | [NISAR](https://www.earthdata.nasa.gov/news/nisar-l-band-data-released-expanding-record-surface-changes) | SPECIALIST | ASF/Earthdata for L-band; Bhoonidhi catalog route for S-band | Track provisional surface-change products; no pre-2024 disaster history, universal millimetre warning, or slope-safety claim. |

### 5.3 Terrain, buildings, population, land cover, soil, and climate

| ID | Source/product | Class | Acquisition route | Intended use and controlling qualification |
| --- | --- | --- | --- | --- |
| S18 | [Copernicus DEM GLO-30](https://dataspace.copernicus.eu/explore-data/data-collections/copernicus-contributing-missions/collections-description/COP-DEM) | CORE | CDSE/Earth Search/verified mirror | Regional terrain screening; a DSM is not bare earth, soil thickness, survey accuracy, or engineering evidence. |
| S19 | [CartoDEM/Cartosat-1 DSM](https://www.nrsc.gov.in/nrscnew/Dataproducts_Thematic_cartodem.php) | ALTERNATIVE | Bhuvan/Bhoonidhi browse-and-order; confirm collection/delivery | Indian terrain alternative; do not transfer flat-terrain accuracy claims to steep ground or infer strength. |
| S20 | [SRTM](https://developers.google.com/earth-engine/datasets) | ALTERNATIVE | NASA/USGS or approved catalog route | Older regional comparison only; pin release and do not treat several DEM mirrors as independent safety evidence. |
| S21 | [JAXA AW3D30](https://www.eorc.jaxa.jp/ALOS/en/dataset/aw3d30/aw3d30_e.htm) | ALTERNATIVE | Current JAXA registration/download process | Independent DSM comparison; not soil/bedrock measurement and still subject to void-fill/canopy bias. |
| S22 | [Google Open Buildings V3/temporal](https://sites.research.google/gr/open-buildings/) | CORE | Permitted direct/Cloud Storage download or eligible Earth Engine use | Footprint/exposure evidence; not a post-2024 Wayanad census, occupancy, residential use, identity, or ownership. |
| S23 | [Microsoft Global ML Building Footprints](https://github.com/microsoft/GlobalMLBuildingFootprints) | ALTERNATIVE | Tile-index downloads or approved mirror | Completeness cross-check; pin release/license and deduplicate against Google/OSM rather than summing. |
| S24 | [Meta/CIESIN HRSL](https://ai.meta.com/ai-for-good/docs/high-resolution-population-density-maps-demographic-estimates-documentation/) | ALTERNATIVE | HDX/open-data S3; optional billed Athena | Modelled sensitivity check only; small cells are not precise counts and no India artifact was acquired in the audit. |
| S25 | [Census 2011 PCA/Wayanad DCHB](https://censusindia.gov.in/nada/index.php/catalog/study/DH_2011_3203_PART_B_DCHB_WAYANAD) | CORE | Public Census catalogs/indicator interface/files | Official baseline aggregates; reconcile 2011 codes to LGD and do not infer individual households or unsupplied vulnerability fields. |
| S26 | [WorldPop Global2](https://www.worldpop.org/blog/worldpop-global2-global-high-resolution-population-estimates-for-2015-2030) ([STAC](https://stac.worldpop.org/)) | SUPPORT | Download hub/public STAC | Recent modelled population context; not a census/beneficiary register and not automatically averaged with correlated products. |
| S27 | [JRC GHSL](https://human-settlement.emergency.copernicus.eu/download.php) | ALTERNATIVE | Public raster/vector download | Settlement/built-up cross-check; future epochs are projections and built-up area is not population. |
| S28 | [ESA WorldCover](https://esa-worldcover.org/en/data-access) | CORE | Public GeoTIFF/COG/AWS or approved mirror | Land-cover screening; mapped tree cover is not notified forest or proof of livelihood rights. |
| S29 | [Dynamic World V1](https://developers.google.com/earth-engine/datasets/catalog/GOOGLE_DYNAMICWORLD_V1) | OPTIONAL | Earth Engine collection subject to approved terms | Frequent probability/change context; shares Sentinel-2 input and cannot establish occupancy or legal use. |
| S30 | [JRC Global Surface Water](https://global-surface-water.appspot.com/download) | SUPPORT | Direct files/explorer/web services | Historical water occurrence/seasonality; not discharge, potable yield, or flood depth. Check version-specific recurrence limitations. |
| S31 | [ISRIC SoilGrids](https://docs.isric.org/globaldata/soilgrids/) ([WCS](https://docs.isric.org/globaldata/soilgrids/wcs.html)) | OPTIONAL | WCS or raster files; REST is not a dependency while paused | Regional soil context only; does not supply measured strength, slip depth, root reinforcement, or bearing capacity. |
| S32 | [IMD historical gridded rainfall](https://www.imdpune.gov.in/cmpg/Griddata/Rainfall_25_NetCDF.html) | SUPPORT | Public IMD Pune files | Climatology/accumulation context; record actual time convention, missing flags, and period. Not local cloudburst telemetry. |
| S33 | [IMD operational APIs](https://api.imd.gov.in/) | OPTIONAL API | Registration and current entitlement/authentication/whitelisting review | Official observations/forecasts where permitted; district averages do not establish site rainfall or stability. |
| S34 | [NASA GPM IMERG V07](https://gpm.nasa.gov/data/imerg) | SUPPORT | Earthdata/CMR/GES DISC, usually Earthdata Login | Blended precipitation estimate; convert rate to interval depth and account for terrain/orographic error. |
| S35 | [CHIRPS v3](https://www.chc.ucsb.edu/data/chirps3) | SUPPORT | Public file automation | Long seasonal rainfall/drought context; choose temporal product and do not treat it as hourly gauge truth or independent where inputs overlap. |
| S36 | [ERA5-Land/CDS](https://cds.climate.copernicus.eu/datasets/reanalysis-era5-land?tab=overview) | OPTIONAL | CDS account, dataset license, API token/download | Reanalysis context only; accumulated variables need differencing and soil-water fields are not site pore pressure. |
| S37 | [NASA SMAP enhanced soil moisture](https://nsidc.org/data/smap/data) | SPECIALIST | NSIDC/Earthdata account and CMR tooling | Regional wetness context with QA; not slip-surface saturation and constrained by terrain/vegetation and effective resolution. |

### 5.4 Water, access, and official geography

| ID | Source/product | Class | Acquisition route | Intended use and controlling qualification |
| --- | --- | --- | --- | --- |
| S38 | [CGWB NAQUIM Wayanad](https://cgwb.gov.in/old_website/AQM/NAQUIM_REPORT/Kerala/WAYANAD%20DISTRICT%20KR.pdf) | CORE CONTEXT | Public reports; request borehole/test data from custodians | Aquifer/regional context; no universal parcel-yield API, and water level/aquifer averages are not sustainable discharge. |
| S39 | [NWIC NWDP/India-WRIS](https://www.nwdp.nwic.gov.in/dataset/ground-water-level-manual-quarterly-kerala-ground-water) ([API catalog](https://nwdp.nwic.gov.in/dataset_api/home_api_page)) | SUPPORT API | Public CSV/resource-specific API where available | Nearby water series after station/schema/missingness review; listing does not guarantee a site gauge and level is not litres/day. |
| S40 | [HydroSHEDS/HydroBASINS/HydroRIVERS](https://www.hydrosheds.org/products/hydrobasins) | SUPPORT | Public regional download | Catchment/connectivity context; not surveyed channels, culverts, discharge, or approved runout/flood footprints. |
| S41 | [OpenStreetMap/Geofabrik](https://download.geofabrik.de/asia/india/southern-zone.html) | CORE | Permitted PBF for batch; small permitted Overpass queries | Road/path/building/POI context; field-verify topology, capacity, legal access, closures, and facility operation. Do not bulk-download public tiles. |
| S42 | [PMGSY GeoSadak](https://pmgsygeosadak.dord.gov.in/OpenData) | CORE | State/layer shapefile downloads | Rural roads/habitations/facilities complement; IDs are not Census/LGD codes and coverage/operation require validation. |
| S43 | [Local Government Directory](https://lgdirectory.gov.in/downloadDirectory.do) | CORE | Directory downloads; registered/keyed APIs where applicable | Current codes/hierarchy and change history; use reviewed crosswalks and do not conflate village, habitation, ward, or boundary geometry. |
| S44 | [Survey of India boundaries](https://surveyofindia.gov.in/pages/village-boundary-data-base-of-entire-india) | CORE | SOI/Online Maps process; product-dependent registration/category | Authoritative administrative geometry; not ownership parcels. Pin edition/CRS and reconcile Census/LGD codes. |

### 5.5 Restricted agency and field blockers

| ID | Source/product | Class | Acquisition route | Intended use and controlling qualification |
| --- | --- | --- | --- | --- |
| S45 | [Kerala Ente Bhoomi/ReLIS/Survey-Revenue-Registration](https://entebhoomi.kerala.gov.in/) | AGENCY BLOCKER | Citizen portals plus agency-approved bulk export/integration | Parcel, resurvey, RoR/tax/registration evidence; DILRMP/ULPIN is not a clean-title feed. Reconcile disputes, occupation, and legal review. |
| S46 | [JJM/Kerala Water Authority/local operator](https://jaljeevanmission.gov.in/about_jjm) ([KWA GIS](https://kwa.kerala.gov.in/ml/gis/)) | AGENCY BLOCKER | Public reports plus signed engineering/operating records | Scheme/network/capacity evidence; a dashboard connection does not prove spare capacity, quality, yield, or feasibility. |
| S47 | [Forest/FRA/protected-area/local environmental records](https://tribal.nic.in/FRA/data/FRARulesBook.pdf) | AGENCY BLOCKER | Competent Forest/Revenue/Tribal departments and lawful community process | Restrictions, claims, decisions, and access rights; tree cover or absent digital record never proves absence of rights. |
| S48 | Household enumeration/local registers/participation | FIELD BLOCKER | Authorized local records, interviews, and secure offline survey | Actual household, needs, preferences, and consent evidence; never derive a register from roofs/proxies or mass-scrape identity/health data. |
| S49 | Qualified site surveys, geotechnics, water tests, and ground controls | FIELD BLOCKER | Commission competent teams; import signed measurements/reports | Operational engineering parameters and lean-season supply evidence; satellite/geology/drone proxies cannot replace qualified measurement. |
| S50 | [Programme eligibility, funding, sanctions, and completion records](https://sdma.kerala.gov.in/vulnerability-linked-relocation-scheme/) | AGENCY BLOCKER | Current orders and authorized finance/project/inspection records | Distinguish announced, sanctioned, received, ready, handed over, and occupied states; public progress is not household readiness. |

### 5.6 Geography-conditional, display, and paid options

| ID | Source/product | Class | Acquisition route | Intended use and controlling qualification |
| --- | --- | --- | --- | --- |
| S51 | [USGS earthquake feeds](https://earthquake.usgs.gov/earthquakes/feed/v1.0/geojson.php) / [NCS records](https://seismo.gov.in/data-portal) | LEGACY/OPTIONAL | Public USGS feeds; NCS catalog/formal request as applicable | Specialist history/event context only; magnitude is not site motion or a safe/unsafe radius and this is not baseline scope. |
| S52 | CWC/ISRO Himalayan glacial-lake products via [Bhuvan](https://bhuvan.nrsc.gov.in/wiki/index.php/How_to_use_WMS_services) | GEOGRAPHY CONDITIONAL | WMS/thematic download or authoritative agency assessment | Himalayan-only screening; lake outline is not volume, breach hydrograph, or inundation. Not part of Wayanad baseline. |
| S53 | [CARTO](https://docs.carto.com/carto-for-developers/key-concepts/carto-for-deck.gl/basemaps/carto-basemap) / [MapTiler](https://www.maptiler.com/terms/cloud) / licensed self-hosted OSM basemap | DISPLAY ONLY | Provider account/key or tiles built from licensed extracts | Reference cartography, never analytical evidence. Approve attribution, quotas, offline/cache rights, privacy, and fallback; public OSM tile bulk download is prohibited. |
| S54 | [Licensed high-resolution imagery](https://docs.planet.com/develop/apis/) or permitted drone/ground survey | OPTIONAL PAID | Procured license/order/subscription or authorized survey | Investigate parcel discrepancies after a defined gap; higher resolution still cannot prove occupation, ownership, consent, or foundations. |

## 6. Source activation and acquisition gate

Every required input is mapped to one or more `S*` entries and an acquisition method. A source version can move from `REGISTERED` to `USABLE` only after a permitted AOI sample records publisher/custodian, stable product/release, source URL, license plus redistribution/offline rights, acquisition and observation/valid time, extent, schema/format, CRS and vertical datum, resolution/scale, units and NoData, uncertainty/quality, checksum, expected cadence/latency, authentication/quota/cost, fallback, owner, and reviewer.

The gate must prove download or governed receipt, parsing, AOI/time coverage, plausible values/units, geometry/raster validity, permitted use, and reproducible processing. `DOCUMENTED` or `CATALOG_VISIBLE` is not `USABLE`. A stale, failed, inaccessible, out-of-coverage, or mandatory missing input produces `UNKNOWN`/`HOLD`; it never yields `PASS`.

Restricted parcel/household/field records additionally require approved purpose, lawful basis, audience, access controls, retention, and disclosure rules. Real site approval or household allocation remains disabled until S45–S50 evidence is obtained and reviewed. Public/open S01–S44 may support the bounded mapping/exposure sandbox with synthetic households and sample parcels.

## 7. Recommended Wayanad prototype stack

Use SOI/LGD geography (S43–S44), KSDMA/GSI hazard evidence (S01–S04), one reviewed terrain source (S18 or S19), Sentinel-2 through CDSE with an approved alternate (S08, S10–S12), Open Buildings with selected OSM/Microsoft checks (S22–S23, S41), Census with clearly modelled WorldPop context if needed (S25–S26), WorldCover (S28), OSM/PMGSY access (S41–S42), NAQUIM/NWDP context (S38–S39), and reviewed IMD/IMERG/CHIRPS history (S32, S34–S35). Do not average mirrors or correlated products as though they were independent observations.

## 8. Specialist scientific references

The specialist models retained in [equations.md](./equations.md) cite USGS, HEC-HMS, HEC-RAS, RAMMS and peer-reviewed root-reinforcement material next to the relevant equation. These sources describe methods; they do not validate a locally calibrated Wayanad implementation.

## 9. Unresolved evidence items

1. Obtain the consolidated amended Disaster Management Act and map every relied-on section, not only §31(4).
2. Capture the exact Gazette commencement instrument and every operative Kerala programme/order used by the pilot.
3. Obtain written current coverage/terms for every hazard product and integration; do not assume C-FLOOD expansion.
4. Obtain the official pilot cadastral, household, water, service, funding and appeals data agreements.
5. Verify the original publication, unit convention and applicability of the transcript-attributed GLOF coefficient before retaining it anywhere beyond `UNVERIFIED` history.
6. Select and license a basemap whose attribution and offline use are compatible with the pilot.
7. Confirm DPDP/CERT-In provisions and commencement applicable to the operating entity on the deployment date.
8. Register the actual SIH/problem statement if competition compliance remains a goal; it was not supplied for this baseline.

9. Obtain and validate the minimum-stack AOI samples; no source in S01–S54 has passed the operational acquisition gate solely by inclusion here.
10. Obtain S45–S50 agency and field evidence before enabling real approval, allocation, handover, or completion claims.

## 10. Evidence-change protocol

An evidence update creates a new `SRC-*` revision and linked dataset/legal-register version, states what changed, identifies affected rules/formulas/results, and records reviewer approval. A broken or inaccessible link does not make a cached claim current. Claims remain qualified by jurisdiction and effective date.
