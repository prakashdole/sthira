package org.sthira.mobile.android.ui

import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.sthira.mobile.android.accessibility.accessibleTarget
import org.sthira.mobile.android.theme.SthiraBluePrimary

data class SupportedLanguage(
    val code: String,
    val vernacularName: String,
    val englishName: String,
    val isPrimaryInState: Boolean
)

val AVAILABLE_LANGUAGES = listOf(
    SupportedLanguage("ml", "മലയാളം", "Malayalam (Kerala)", true),
    SupportedLanguage("hi", "हिन्दी", "Hindi (National / Regional)", true),
    SupportedLanguage("en", "English", "English (Official Fallback)", false)
)

@Composable
fun LanguageScreen(
    currentLanguageCode: String,
    onLanguageSelected: (String) -> Unit,
    modifier: Modifier = Modifier
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .padding(20.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally
    ) {
        Text(
            text = "Select Language / ഭാഷ തിരഞ്ഞെടുക്കുക",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface
        )

        Spacer(modifier = Modifier.height(8.dp))

        Text(
            text = "Choose your preferred language for voice and emergency alerts",
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )

        Spacer(modifier = Modifier.height(28.dp))

        AVAILABLE_LANGUAGES.forEach { lang ->
            LanguageOptionCard(
                language = lang,
                isSelected = currentLanguageCode == lang.code,
                onClick = { onLanguageSelected(lang.code) }
            )
            Spacer(modifier = Modifier.height(14.dp))
        }
    }
}

@Composable
private fun LanguageOptionCard(
    language: SupportedLanguage,
    isSelected: Boolean,
    onClick: () -> Unit
) {
    val borderColor = if (isSelected) SthiraBluePrimary else MaterialTheme.colorScheme.outlineVariant

    Card(
        modifier = Modifier
            .fillMaxWidth()
            .accessibleTarget(56)
            .border(width = if (isSelected) 2.dp else 1.dp, color = borderColor, shape = RoundedCornerShape(12.dp))
            .clickable(onClick = onClick)
            .semantics {
                contentDescription = "${language.vernacularName}, ${language.englishName}. ${if (isSelected) "Selected" else "Not selected"}"
            },
        colors = CardDefaults.cardColors(
            containerColor = if (isSelected) MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.15f)
            else MaterialTheme.colorScheme.surface
        ),
        shape = RoundedCornerShape(12.dp)
    ) {
        Row(
            modifier = Modifier
                .padding(16.dp)
                .fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically
        ) {
            RadioButton(
                selected = isSelected,
                onClick = onClick,
                modifier = Modifier.accessibleTarget(48)
            )

            Spacer(modifier = Modifier.width(12.dp))

            Column {
                Text(
                    text = language.vernacularName,
                    fontSize = 18.sp,
                    fontWeight = FontWeight.Bold,
                    color = MaterialTheme.colorScheme.onSurface
                )
                Text(
                    text = language.englishName,
                    fontSize = 13.sp,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}
