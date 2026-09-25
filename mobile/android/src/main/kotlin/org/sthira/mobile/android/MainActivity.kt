package org.sthira.mobile.android

import android.Manifest
import android.content.pm.PackageManager
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.core.content.ContextCompat
import org.sthira.mobile.android.accessibility.accessibleTarget
import org.sthira.mobile.android.theme.SthiraTheme
import org.sthira.mobile.android.ui.DestinationPickerScreen
import org.sthira.mobile.android.ui.IncidentOverviewScreen
import org.sthira.mobile.android.ui.LanguageScreen
import org.sthira.mobile.android.ui.RouteGuidanceScreen
import org.sthira.mobile.android.ui.StayManagementScreen
import org.sthira.mobile.android.viewmodel.CitizenJourneyViewModel

enum class NavigationScreen(val title: String) {
    INCIDENT("Alerts"),
    DESTINATIONS("Shelters"),
    ROUTE("Guidance"),
    STAY("My Stay"),
    LANGUAGE("Language")
}

class MainActivity : ComponentActivity() {

    private val viewModel: CitizenJourneyViewModel by viewModels()

    private val permissionLauncher = registerForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions()
    ) { permissions ->
        val micGranted = permissions[Manifest.permission.RECORD_AUDIO] ?: false
        val locGranted = permissions[Manifest.permission.ACCESS_FINE_LOCATION] ?: false
        // Non-intrusive: if denied, text fallback & manual non-map navigation remain fully usable
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // Request runtime permissions for voice & foreground guidance
        checkAndRequestPermissions()

        setContent {
            SthiraTheme {
                SthiraApp(viewModel = viewModel)
            }
        }
    }

    private fun checkAndRequestPermissions() {
        val permissionsToRequest = mutableListOf<String>()
        if (ContextCompat.checkSelfPermission(this, Manifest.permission.RECORD_AUDIO) != PackageManager.PERMISSION_GRANTED) {
            permissionsToRequest.add(Manifest.permission.RECORD_AUDIO)
        }
        if (ContextCompat.checkSelfPermission(this, Manifest.permission.ACCESS_FINE_LOCATION) != PackageManager.PERMISSION_GRANTED) {
            permissionsToRequest.add(Manifest.permission.ACCESS_FINE_LOCATION)
        }
        if (permissionsToRequest.isNotEmpty()) {
            permissionLauncher.launch(permissionsToRequest.toTypedArray())
        }
    }
}

@Composable
fun SthiraApp(viewModel: CitizenJourneyViewModel) {
    val uiState by viewModel.uiState.collectAsState()
    var currentScreen by remember { mutableStateOf(NavigationScreen.INCIDENT) }
    val snackbarHostState = remember { SnackbarHostState() }
    val context = LocalContext.current

    LaunchedEffect(uiState.statusMessage) {
        uiState.statusMessage?.let {
            snackbarHostState.showSnackbar(it)
        }
    }

    Scaffold(
        snackbarHost = { SnackbarHost(snackbarHostState) },
        bottomBar = {
            NavigationBar {
                NavigationScreen.values().forEach { screen ->
                    val isSelected = currentScreen == screen
                    NavigationBarItem(
                        selected = isSelected,
                        onClick = { currentScreen = screen },
                        icon = {
                            Text(
                                text = when (screen) {
                                    NavigationScreen.INCIDENT -> "🚨"
                                    NavigationScreen.DESTINATIONS -> "⛺"
                                    NavigationScreen.ROUTE -> "🗺️"
                                    NavigationScreen.STAY -> "📋"
                                    NavigationScreen.LANGUAGE -> "🌐"
                                }
                            )
                        },
                        label = { Text(screen.title) },
                        modifier = Modifier
                            .accessibleTarget(48)
                            .semantics {
                                contentDescription = "Navigate to ${screen.title} screen. ${if (isSelected) "Currently active" else ""}"
                            }
                    )
                }
            }
        }
    ) { innerPadding ->
        Box(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
        ) {
            when (currentScreen) {
                NavigationScreen.INCIDENT -> {
                    IncidentOverviewScreen(
                        incident = uiState.incident,
                        isSyntheticExercise = uiState.isSyntheticExercise,
                        onNavigateToDestinations = { currentScreen = NavigationScreen.DESTINATIONS }
                    )
                }
                NavigationScreen.DESTINATIONS -> {
                    DestinationPickerScreen(
                        facilities = uiState.facilities,
                        ambiguousCandidates = uiState.ambiguousCandidates,
                        isVoiceRecording = uiState.isVoiceRecording,
                        isProcessing = uiState.isProcessing,
                        onVoiceClick = { viewModel.toggleVoiceInput() },
                        onTextSearch = { query -> viewModel.onTextSearch(query) },
                        onCandidateSelected = { candidate -> viewModel.selectCandidate(candidate) },
                        onFacilitySelected = { facility ->
                            viewModel.selectFacility(facility)
                            currentScreen = NavigationScreen.ROUTE
                        }
                    )
                }
                NavigationScreen.ROUTE -> {
                    val facility = uiState.selectedFacility ?: uiState.facilities.firstOrNull()
                    if (facility != null) {
                        RouteGuidanceScreen(
                            facility = facility,
                            route = uiState.activeRoute,
                            instructions = uiState.instructions,
                            isRouteRevoked = uiState.isRouteRevoked,
                            onStartNavigation = { viewModel.startTracking(context) },
                            onManageStay = { currentScreen = NavigationScreen.STAY }
                        )
                    } else {
                        DestinationPickerScreen(
                            facilities = uiState.facilities,
                            ambiguousCandidates = uiState.ambiguousCandidates,
                            isVoiceRecording = uiState.isVoiceRecording,
                            isProcessing = uiState.isProcessing,
                            onVoiceClick = { viewModel.toggleVoiceInput() },
                            onTextSearch = { query -> viewModel.onTextSearch(query) },
                            onCandidateSelected = { candidate -> viewModel.selectCandidate(candidate) },
                            onFacilitySelected = { selected ->
                                viewModel.selectFacility(selected)
                            }
                        )
                    }
                }
                NavigationScreen.STAY -> {
                    val facility = uiState.selectedFacility ?: uiState.facilities.firstOrNull()
                    if (facility != null) {
                        StayManagementScreen(
                            facility = facility,
                            currentReservation = uiState.activeReservation,
                            isOffline = uiState.isOffline,
                            isSubmitting = uiState.isProcessing,
                            onConfirmReservation = { count -> viewModel.reserveShelter(count) },
                            onExplicitArrival = { resId -> viewModel.confirmExplicitArrival(resId) },
                            onExtendStay = { resId -> viewModel.extendStay(resId) },
                            onDepartStay = { resId -> viewModel.departStay(resId) },
                            onTransferStay = { resId, newFacId -> },
                            onCancelStay = { resId -> viewModel.cancelStay(resId) }
                        )
                    } else {
                        Text(
                            text = "Please select a shelter facility first.",
                            modifier = Modifier.padding(16.dp)
                        )
                    }
                }
                NavigationScreen.LANGUAGE -> {
                    LanguageScreen(
                        currentLanguageCode = uiState.selectedLanguage,
                        onLanguageSelected = { langCode ->
                            viewModel.selectLanguage(langCode)
                        }
                    )
                }
            }
        }
    }
}
