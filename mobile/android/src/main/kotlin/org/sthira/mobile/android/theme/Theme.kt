package org.sthira.mobile.android.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable

private val DarkColorScheme = darkColorScheme(
    primary = SthiraBlueLight,
    onPrimary = SthiraSurfaceLight,
    primaryContainer = SthiraBlueDark,
    onPrimaryContainer = SthiraSurfaceLight,
    secondary = SafeGreen,
    onSecondary = SthiraSurfaceLight,
    surface = SthiraSurfaceDark,
    onSurface = SthiraTextPrimaryDark,
    background = SthiraBackgroundDark,
    onBackground = SthiraTextPrimaryDark,
    error = HazardRed,
    onError = SthiraSurfaceLight
)

private val LightColorScheme = lightColorScheme(
    primary = SthiraBluePrimary,
    onPrimary = SthiraSurfaceLight,
    primaryContainer = SthiraBlueLight,
    onPrimaryContainer = SthiraBlueDark,
    secondary = SafeGreen,
    onSecondary = SthiraSurfaceLight,
    surface = SthiraSurfaceLight,
    onSurface = SthiraTextPrimaryLight,
    background = SthiraBackgroundLight,
    onBackground = SthiraTextPrimaryLight,
    error = HazardRed,
    onError = SthiraSurfaceLight
)

@Composable
fun SthiraTheme(
    darkTheme: Boolean = isSystemInDarkTheme(),
    content: @Composable () -> Unit
) {
    val colorScheme = if (darkTheme) DarkColorScheme else LightColorScheme

    MaterialTheme(
        colorScheme = colorScheme,
        typography = SthiraTypography,
        content = content
    )
}
