package org.sthira.mobile.android.journey

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.location.Location
import android.location.LocationListener
import android.location.LocationManager
import android.os.Binder
import android.os.Build
import android.os.Bundle
import android.os.IBinder
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import org.sthira.mobile.android.MainActivity
import org.sthira.mobile.contracts.JourneyState
import org.sthira.mobile.contracts.JourneyStateEngine
import org.sthira.mobile.contracts.MonotonicClock
import org.sthira.mobile.contracts.PositionReading

/**
 * ForegroundJourneyService: an explicit foreground service that guides the citizen to their
 * designated safe facility.
 *
 * Invariants (O10):
 * - Citizen explicitly starts and stops the service.
 * - Displays an ongoing persistent notification showing distance and manual arrival requirement.
 * - Zero background location surveillance: halts immediately when user stops or cancels navigation.
 * - Never uploads continuous coordinate traces to any remote server.
 */
class ForegroundJourneyService : Service(), LocationListener {

    companion object {
        const val CHANNEL_ID = "sthira_journey_guidance"
        const val NOTIFICATION_ID = 4040
        const val ACTION_START_GUIDANCE = "org.sthira.mobile.action.START_GUIDANCE"
        const val ACTION_STOP_GUIDANCE = "org.sthira.mobile.action.STOP_GUIDANCE"
        const val EXTRA_DEST_LAT = "dest_lat"
        const val EXTRA_DEST_LON = "dest_lon"
        const val EXTRA_DEST_NAME = "dest_name"

        fun startService(context: Context, destLat: Double, destLon: Double, destName: String) {
            val intent = Intent(context, ForegroundJourneyService::class.java).apply {
                action = ACTION_START_GUIDANCE
                putExtra(EXTRA_DEST_LAT, destLat)
                putExtra(EXTRA_DEST_LON, destLon)
                putExtra(EXTRA_DEST_NAME, destName)
            }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }

        fun stopService(context: Context) {
            val intent = Intent(context, ForegroundJourneyService::class.java).apply {
                action = ACTION_STOP_GUIDANCE
            }
            context.startService(intent)
        }
    }

    private val binder = LocalBinder()
    private val serviceScope = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    private var locationManager: LocationManager? = null

    private var destinationLat: Double = 0.0
    private var destinationLon: Double = 0.0
    private var destinationName: String = "Designated Shelter"

    private val engine = JourneyStateEngine()
    private val _serviceJourneyState = MutableStateFlow(JourneyState.NOT_STARTED)
    val serviceJourneyState: StateFlow<JourneyState> = _serviceJourneyState.asStateFlow()

    private val _lastReading = MutableStateFlow<PositionReading?>(null)
    val lastReading: StateFlow<PositionReading?> = _lastReading.asStateFlow()

    private val _distanceMeters = MutableStateFlow<Double?>(null)
    val distanceMeters: StateFlow<Double?> = _distanceMeters.asStateFlow()

    inner class LocalBinder : Binder() {
        fun getService(): ForegroundJourneyService = this@ForegroundJourneyService
    }

    override fun onBind(intent: Intent?): IBinder = binder

    override fun onCreate() {
        super.onCreate()
        locationManager = getSystemService(Context.LOCATION_SERVICE) as LocationManager
        createNotificationChannel()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START_GUIDANCE -> {
                destinationLat = intent.getDoubleExtra(EXTRA_DEST_LAT, 0.0)
                destinationLon = intent.getDoubleExtra(EXTRA_DEST_LON, 0.0)
                destinationName = intent.getStringExtra(EXTRA_DEST_NAME) ?: "Designated Shelter"

                val notification = buildNotification("Evacuation guidance active", "Heading to $destinationName")
                startForeground(NOTIFICATION_ID, notification)

                engine.startTracking(destinationLat, destinationLon)
                _serviceJourneyState.value = engine.currentState
                startLocationUpdates()
            }
            ACTION_STOP_GUIDANCE -> {
                stopGuidance()
            }
        }
        return START_NOT_STICKY
    }

    private fun startLocationUpdates() {
        try {
            locationManager?.requestLocationUpdates(
                LocationManager.GPS_PROVIDER,
                2000L, // 2s intervals
                5f,    // 5m minimum displacement
                this
            )
        } catch (_: SecurityException) {
            _serviceJourneyState.value = JourneyState.LOCATION_UNAVAILABLE
            updateNotification("Location permission required", "Please enable foreground location")
        }
    }

    override fun onLocationChanged(location: Location) {
        val nowEpoch = System.currentTimeMillis()
        val reading = PositionReading(
            latitude = location.latitude,
            longitude = location.longitude,
            accuracyMeters = location.accuracy.toDouble(),
            timestampEpochMs = nowEpoch,
            monotonicElapsedMs = MonotonicClock.currentMonotonicMs()
        )
        _lastReading.value = reading

        val newState = engine.updatePosition(reading)
        _serviceJourneyState.value = newState

        val dist = engine.calculateDistanceMeters(reading.latitude, reading.longitude, destinationLat, destinationLon)
        _distanceMeters.value = dist

        val distText = if (dist >= 1000) "%.1f km".format(dist / 1000.0) else "%d m".format(dist.toInt())

        val contentText = when (newState) {
            JourneyState.NEAR_DESTINATION -> "Near destination ($distText). Touch below to confirm arrival."
            JourneyState.TRACKING -> "Distance to $destinationName: $distText"
            JourneyState.LOCATION_UNAVAILABLE -> "Poor GPS signal — advisory guidance only"
            else -> "Navigating to $destinationName"
        }
        updateNotification("Evacuation Navigation", contentText)
    }

    fun stopGuidance() {
        try {
            locationManager?.removeUpdates(this)
        } catch (_: Exception) {}
        engine.stopTracking()
        _serviceJourneyState.value = engine.currentState
        stopForeground(true)
        stopSelf()
    }

    override fun onDestroy() {
        super.onDestroy()
        stopGuidance()
        serviceScope.cancel()
    }

    override fun onStatusChanged(provider: String?, status: Int, extras: Bundle?) {}
    override fun onProviderEnabled(provider: String) {}
    override fun onProviderDisabled(provider: String) {
        _serviceJourneyState.value = JourneyState.LOCATION_UNAVAILABLE
        updateNotification("GPS Disabled", "Please enable GPS for evacuation route guidance")
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                "Sthira Evacuation Guidance",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "Shows live progress to your designated shelter during an evacuation"
            }
            val manager = getSystemService(NotificationManager::class.java)
            manager.createNotificationChannel(channel)
        }
    }

    private fun buildNotification(title: String, content: String): Notification {
        val openIntent = Intent(this, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_SINGLE_TOP
        }
        val openPendingIntent = PendingIntent.getActivity(
            this,
            0,
            openIntent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )

        val stopIntent = Intent(this, ForegroundJourneyService::class.java).apply {
            action = ACTION_STOP_GUIDANCE
        }
        val stopPendingIntent = PendingIntent.getService(
            this,
            1,
            stopIntent,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
        )

        val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL_ID)
        } else {
            @Suppress("DEPRECATION")
            Notification.Builder(this)
        }

        return builder
            .setContentTitle(title)
            .setContentText(content)
            .setSmallIcon(android.R.drawable.ic_menu_directions)
            .setContentIntent(openPendingIntent)
            .addAction(
                Notification.Action.Builder(
                    android.R.drawable.ic_delete,
                    "Stop Navigation",
                    stopPendingIntent
                ).build()
            )
            .setOngoing(true)
            .build()
    }

    private fun updateNotification(title: String, content: String) {
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        manager.notify(NOTIFICATION_ID, buildNotification(title, content))
    }
}
