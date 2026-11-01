package ke.co.baharilink.baharimrv.ui.components

import ke.co.baharilink.baharimrv.data.remote.AmountDto
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class FormattingTest {
    @Test
    fun ledgerLabelsNeverInventIotaForSandboxData() {
        assertEquals("IOTA testnet", chainNetworkLabel("iota_testnet"))
        assertEquals("IOTA mainnet", chainNetworkLabel("iota_mainnet"))
        assertEquals("Sandbox ledger", chainNetworkLabel("sandbox"))
        assertEquals("Sandbox ledger", chainNetworkLabel("some_future_mode"))
        assertEquals("Ledger mode unavailable", chainNetworkLabel(null))
        assertEquals("Ledger mode unavailable", chainNetworkLabel(""))
    }

    @Test
    fun amountsStayDecimalAndNeverUseExponentialNotation() {
        assertEquals("1234.56 KES", formatAmount(AmountDto("1234.560000", "KES")))
        assertEquals("1000000 KES", formatAmount(AmountDto("1000000", "KES")))
        assertEquals("0.1 USD", formatAmount(AmountDto("0.10", "USD")))
        assertEquals("Unavailable", formatAmount(null))
    }

    @Test
    fun instantsRenderOrFallBackToTheRawValue() {
        assertEquals("timestamp unavailable", formatInstant(null))
        assertEquals("not-a-timestamp", formatInstant("not-a-timestamp"))
        assertTrue(formatInstant("2024-10-26T18:13:20Z").contains("2024"))
    }
}
