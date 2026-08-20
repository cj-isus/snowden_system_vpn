package com.snowden.system.snowden_android

import android.util.Log
import io.nekohasekai.libbox.CommandServerHandler
import io.nekohasekai.libbox.SystemProxyStatus

/**
 * Bridges libbox's CommandServer callbacks into SnowdenVpnService.
 *
 * libbox v1.14.0-lx.3 drives its lifecycle through this handler: when a
 * command client asks for serviceReload or serviceStop, libbox invokes
 * these methods synchronously. The host Android service wires the
 * reload/stop implementations through [Callback] at construction time,
 * so that the handler is decoupled from the VpnService class itself.
 *
 * Methods that don't apply on Android (SSH agent, native crash harness)
 * are no-ops or fixed-value returns so libbox can proceed without
 * throwing.
 */
class SnowdenCommandServerHandler(
    private val callback: Callback
) : CommandServerHandler {

    interface Callback {
        /** Called when libbox asks to reload the active profile. */
        fun onServiceReload()
        /** Called when libbox requests full stop. */
        fun onServiceStop()
    }

    // -- System proxy state is tracked locally; the VPN itself doesn't
    // -- intercept HTTP traffic on Android, so the value is informational
    // -- only and is displayed by the Flutter UI through info callbacks.

    @Volatile
    private var systemProxyEnabled: Boolean = false

    override fun getSystemProxyStatus(): SystemProxyStatus {
        return SystemProxyStatus().apply {
            available = false
            enabled = systemProxyEnabled
        }
    }

    override fun setSystemProxyEnabled(enabled: Boolean) {
        systemProxyEnabled = enabled
        Log.i(TAG, "setSystemProxyEnabled=$enabled")
    }

    // -- Lifecycle control ------------------------------------------------

    override fun serviceReload() {
        try {
            callback.onServiceReload()
        } catch (t: Throwable) {
            Log.e(TAG, "serviceReload failed", t)
        }
    }

    override fun serviceStop() {
        try {
            callback.onServiceStop()
        } catch (t: Throwable) {
            Log.e(TAG, "serviceStop failed", t)
        }
    }

    // -- SSH / debug / native crash (no-ops) ------------------------------

    override fun connectSSHAgent(): Int = -1

    override fun writeDebugMessage(message: String) {
        Log.d(TAG, "libbox-debug: $message")
    }

    override fun triggerNativeCrash() {
        // Intentional no-op: triggering a native crash in production would
        // brick the user-facing service. Kept as a hook for internal QA
        // builds only.
    }

    companion object {
        private const val TAG = "SnowdenVpn"
    }
}
