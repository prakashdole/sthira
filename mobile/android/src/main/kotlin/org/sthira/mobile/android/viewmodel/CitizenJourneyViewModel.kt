package org.sthira.mobile.android.viewmodel

import android.content.Context
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import org.sthira.mobile.android.audio.EphemeralAudioRecorder
import org.sthira.mobile.android.journey.ForegroundJourneyService
import org.sthira.mobile.contracts.AmbiguousPlaceException
import org.sthira.mobile.contracts.FreshnessState
import org.sthira.mobile.contracts.InstructionStep
import org.sthira.mobile.contracts.JourneyState
import org.sthira.mobile.contracts.JourneyStateEngine
import org.sthira.mobile.contracts.MonotonicClock
import org.sthira.mobile.contracts.PlaceCandidate
import org.sthira.mobile.contracts.PositionReading
import org.sthira.mobile.contracts.ReservationResponse
import org.sthira.mobile.offlinepkg.FacilityCard
import org.sthira.mobile.offlinepkg.PublicIncidentCard
import org.sthira.mobile.offlinepkg.RedZoneCard
import org.sthira.mobile.offlinepkg.RouteCard
import org.sthira.mobile.offlinepkg.SafeZoneCard
import org.sthira.mobile.offlinequeue.DurableOfflineQueue
import org.sthira.mobile.offlinequeue.QueueOpType
import org.sthira.mobile.session.SessionManager
import java.util.UUID

data class CitizenUiState(
    val selectedLanguage: String = "ml",
    val isSyntheticExercise: Boolean = true,
    val incident: PublicIncidentCard? = null,
    val facilities: List<FacilityCard> = emptyList(),
    val ambiguousCandidates: List<PlaceCandidate> = emptyList(),
    val selectedFacility: FacilityCard? = null,
    val activeRoute: RouteCard? = null,
    val instructions: List<InstructionStep> = emptyList(),
    val isRouteRevoked: Boolean = false,
    val activeReservation: ReservationResponse? = null,
    val journeyState: JourneyState = JourneyState.NOT_STARTED,
    val distanceMeters: Double? = null,
    val lastReading: PositionReading? = null,
    val isOffline: Boolean = false,
    val isVoiceRecording: Boolean = false,
    val isProcessing: Boolean = false,
    val statusMessage: String? = null
)

class CitizenJourneyViewModel(
    private val sessionManager: SessionManager? = null,
    private val offlineQueue: DurableOfflineQueue? = null
) : ViewModel() {

    private val _uiState = MutableStateFlow(CitizenUiState())
    val uiState: StateFlow<CitizenUiState> = _uiState.asStateFlow()

    private val journeyEngine = JourneyStateEngine()
    private val audioRecorder = EphemeralAudioRecorder()

    init {
        loadInitialIncidentData()
    }

    private fun loadInitialIncidentData() {
        val demoIncident = PublicIncidentCard(
            id = "INC-KL-2024-WYND-01",
            jurisdiction = "IN-KL-WYND",
            title = "Wayand Landslide & Flash Flood Evacuation Advisory",
            description = "Heavy monsoon rainfall triggered landslides in Meppadi panchayat. All citizens in marked red zones must move immediately to designated safe facilities.",
            issuedAt = "2024-07-30T04:30:00Z",
            effectiveAt = "2024-07-30T04:30:00Z",
            sourceId = "KSDMA-OPERATIONAL",
            sourceVersion = "2024.1",
            redZones = listOf(
                RedZoneCard(
                    id = "RZ-MEPPADI-01",
                    name = "Meppadi Hill Slopes & River Basin",
                    hazardType = "Landslide / Flash Flood",
                    instructions = "Evacuate immediately via approved bypass corridor. Do not use river bridge."
                )
            ),
            safeZones = listOf(
                SafeZoneCard(
                    id = "SZ-VYTHIRI-01",
                    name = "Vythiri High Grounds Safe Zone",
                    facilities = listOf("FAC-VYTHIRI-CH-01", "FAC-MEPPADI-SCH-02")
                )
            )
        )

        val sampleFacilities = listOf(
            FacilityCard(
                id = "FAC-VYTHIRI-CH-01",
                name = "Vythiri Community Hall Shelter",
                safeZoneId = "SZ-VYTHIRI-01",
                latitude = 11.5512,
                longitude = 76.0415,
                capacityTotal = 250,
                capacityRemaining = 78
            ),
            FacilityCard(
                id = "FAC-MEPPADI-SCH-02",
                name = "St. Joseph High School Relief Camp",
                safeZoneId = "SZ-VYTHIRI-01",
                latitude = 11.5480,
                longitude = 76.1280,
                capacityTotal = 400,
                capacityRemaining = 120
            ),
            FacilityCard(
                id = "FAC-KALPETTA-GOVT-03",
                name = "Kalpetta Town Relief Centre",
                safeZoneId = "SZ-VYTHIRI-01",
                latitude = 11.6100,
                longitude = 76.0820,
                capacityTotal = 150,
                capacityRemaining = 0 // Full facility for testing 409 / disabled path
            )
        )

        _uiState.update {
            it.copy(
                incident = demoIncident,
                facilities = sampleFacilities,
                isSyntheticExercise = true
            )
        }
    }

    fun selectLanguage(code: String) {
        _uiState.update { it.copy(selectedLanguage = code) }
    }

    fun toggleVoiceInput() {
        if (_uiState.value.isVoiceRecording) {
            audioRecorder.stop()
            _uiState.update { it.copy(isVoiceRecording = false, isProcessing = true) }
        } else {
            _uiState.update { it.copy(isVoiceRecording = true, isProcessing = false) }
            viewModelScope.launch {
                try {
                    val wavBytes = audioRecorder.recordUtterance()
                    _uiState.update { it.copy(isVoiceRecording = false, isProcessing = true) }
                    // Process voice utterance via IndicConformer ASR (or simulated offline transcription)
                    processVoiceUtterance(wavBytes)
                } catch (e: Exception) {
                    _uiState.update {
                        it.copy(
                            isVoiceRecording = false,
                            isProcessing = false,
                            statusMessage = "Microphone error: ${e.message}"
                        )
                    }
                }
            }
        }
    }

    private fun processVoiceUtterance(wavBytes: ByteArray) {
        // Ephemeral in-memory only: zero disk retention
        _uiState.update {
            it.copy(
                isProcessing = false,
                statusMessage = "Voice processed (16kHz WAV ${wavBytes.size} bytes). Filtering safe destinations."
            )
        }
    }

    fun onTextSearch(query: String) {
        if (query.equals("meppadi", ignoreCase = true)) {
            // Ambiguous place simulation (409 Conflict): returns candidate chips
            val candidates = listOf(
                PlaceCandidate("LOC-MEP-01", "Meppadi Village (Near Church)", 11.5501, 76.1211, 0.95),
                PlaceCandidate("LOC-MEP-02", "Meppadi Junction (Bus Stand)", 11.5542, 76.1265, 0.91)
            )
            _uiState.update { it.copy(ambiguousCandidates = candidates) }
        } else {
            _uiState.update { it.copy(ambiguousCandidates = emptyList()) }
        }
    }

    fun selectCandidate(candidate: PlaceCandidate) {
        _uiState.update {
            it.copy(
                ambiguousCandidates = emptyList(),
                statusMessage = "Location disambiguated: ${candidate.displayName}"
            )
        }
    }

    fun selectFacility(facility: FacilityCard) {
        val sampleRoute = RouteCard(
            id = "RTE-GOV-WYND-001",
            safeZoneId = facility.safeZoneId,
            coordinates = listOf(
                listOf(76.1200, 11.5450),
                listOf(76.1250, 11.5480),
                listOf(76.0415, 11.5512)
            ),
            modes = listOf("FOOT", "BUS")
        )

        val sampleInstructions = listOf(
            InstructionStep("Head northwest on Meppadi Bypass Road", 450, "DEPART"),
            InstructionStep("Turn left at Checkpoint Alpha onto High Ground Corridor", 1200, "TURN_LEFT"),
            InstructionStep("Arrive at ${facility.name} on the right", 150, "ARRIVE")
        )

        _uiState.update {
            it.copy(
                selectedFacility = facility,
                activeRoute = sampleRoute,
                instructions = sampleInstructions,
                isRouteRevoked = false
            )
        }
    }

    fun reserveShelter(peopleCount: Int) {
        val facility = _uiState.value.selectedFacility ?: return
        _uiState.update { it.copy(isProcessing = true) }

        val reservationId = "RES-" + UUID.randomUUID().toString().take(8).uppercase()
        val idempotencyKey = "IDEMP-" + UUID.randomUUID().toString()

        if (_uiState.value.isOffline && offlineQueue != null) {
            // Queue operation durably on device
            viewModelScope.launch {
                offlineQueue.enqueue(
                    QueueOpType.RESERVE_STAY,
                    idempotencyKey,
                    """{"facilityId":"${facility.id}","count":$peopleCount}"""
                )
                _uiState.update {
                    it.copy(
                        isProcessing = false,
                        activeReservation = ReservationResponse(
                            reservationId = reservationId,
                            facilityId = facility.id,
                            allocatedCapacity = peopleCount,
                            status = "PENDING_OFFLINE",
                            expiresAt = "2024-07-31T04:30:00Z",
                            synthetic = true
                        ),
                        statusMessage = "Reservation saved to offline queue."
                    )
                }
            }
        } else {
            // Online reservation confirmed
            _uiState.update {
                it.copy(
                    isProcessing = false,
                    activeReservation = ReservationResponse(
                        reservationId = reservationId,
                        facilityId = facility.id,
                        allocatedCapacity = peopleCount,
                        status = "RESERVED",
                        expiresAt = "2024-07-31T04:30:00Z",
                        synthetic = true
                    ),
                    statusMessage = "Shelter capacity confirmed: $peopleCount beds reserved."
                )
            }
        }
    }

    fun confirmExplicitArrival(reservationId: String) {
        // Explicit touch action: never auto-confirmed by geofence (O10)
        journeyEngine.confirmArrival()
        _uiState.update {
            it.copy(
                journeyState = JourneyState.ARRIVAL_REPORTED,
                activeReservation = it.activeReservation?.copy(status = "ARRIVED"),
                statusMessage = "Arrival confirmed at shelter desk. Capacity updated."
            )
        }
    }

    fun extendStay(reservationId: String) {
        _uiState.update {
            it.copy(
                activeReservation = it.activeReservation?.copy(expiresAt = "2024-08-07T04:30:00Z"),
                statusMessage = "Stay extension approved within 7-day policy limit."
            )
        }
    }

    fun departStay(reservationId: String) {
        journeyEngine.stopTracking()
        _uiState.update {
            it.copy(
                journeyState = JourneyState.NOT_STARTED,
                activeReservation = null,
                selectedFacility = null,
                activeRoute = null,
                statusMessage = "Departed facility. Capacity released back to authority pool."
            )
        }
    }

    fun cancelStay(reservationId: String) {
        journeyEngine.stopTracking()
        _uiState.update {
            it.copy(
                journeyState = JourneyState.NOT_STARTED,
                activeReservation = null,
                selectedFacility = null,
                statusMessage = "Reservation cancelled. Capacity released."
            )
        }
    }

    fun startTracking(context: Context) {
        val facility = _uiState.value.selectedFacility ?: return
        ForegroundJourneyService.startService(
            context = context,
            destLat = facility.latitude,
            destLon = facility.longitude,
            destName = facility.name
        )
        journeyEngine.startTracking(facility.latitude, facility.longitude)
        _uiState.update { it.copy(journeyState = JourneyState.TRACKING) }
    }

    fun stopTracking(context: Context) {
        ForegroundJourneyService.stopService(context)
        journeyEngine.stopTracking()
        _uiState.update { it.copy(journeyState = JourneyState.PAUSED) }
    }

    fun updatePosition(reading: PositionReading) {
        _uiState.update { it.copy(lastReading = reading) }
        val newState = journeyEngine.updatePosition(reading)
        val facility = _uiState.value.selectedFacility
        val dist = if (facility != null) {
            journeyEngine.calculateDistanceMeters(reading.latitude, reading.longitude, facility.latitude, facility.longitude)
        } else null

        _uiState.update {
            it.copy(
                journeyState = newState,
                distanceMeters = dist
            )
        }
    }
}
