package com.snowden.system.snowden_android

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.IBinder
import android.util.Log
import androidx.core.app.NotificationCompat
import io.nekohasekai.libbox.CommandServer
import io.nekohasekai.libbox.Libbox
import io.nekohasekai.libbox.OverrideOptions
import io.nekohasekai.libbox.SetupOptions
import io.nekohasekai.libbox.StringIterator
import java.io.File

/**
 * Foreground VPN service that hosts libbox v1.14.0-lx.3.
 *
 * The previous generation had a one-call lifecycle (Libbox.setup + Libbox.start).
 * The new AAR exposes:
 *   - Libbox.setup(SetupOptions)              global setup, paths, OOM, log limits.
 *   - Libbox.newCommandServer(handler, plat)  command-protocol + service host.
 *   - commandServer.start()                   listen for command clients.
 *   - commandServer.startOrReloadService(json, override)
 *                                             directly load a profile and start
 *                                             the box, bypassing the command
 *                                             protocol. This is the path used
 *                                             by mobile clients.
 *
 * We generate a random listen port and secret at startup so each process
 * boot gets a fresh command-server identity (avoids stale connections
 * after restart).
 */
class SnowdenVpnService : VpnService() {

    companion object {
        const val ACTION_START = "com.snowden.system.START_VPN"
        const val ACTION_STOP = "com.snowden.system.STOP_VPN"
        const val EXTRA_CONFIG = "config"
        const val NOTIFICATION_CHANNEL_ID = "snowden_vpn"
        const val NOTIFICATION_ID = 1

        @JvmStatic
        @Volatile
        var isRunning: Boolean = false

        private const val TAG = "SnowdenVpn"
    }

    private var platformInterface: SnowdenPlatformInterface? = null
    private var commandServer: CommandServer? = null
    private var setupApplied: Boolean = false

    // -- Lifecycle ---------------------------------------------------------

    override fun onCreate() {
        super.onCreate()
        createNotificationChannel()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_START -> {
                val config = intent.getStringExtra(EXTRA_CONFIG) ?: return START_NOT_STICKY
                startVpn(config)
            }
            ACTION_STOP -> {
                stopVpn()
            }
        }
        return START_NOT_STICKY
    }

    override fun onDestroy() {
        stopVpn()
        super.onDestroy()
    }

    // -- Start -------------------------------------------------------------

    private fun startVpn(configJson: String) {
        try {
            if (commandServer != null) {
                Log.w(TAG, "startVpn called while already running")
                return
            }

            applySetupOptionsOnce()

            platformInterface = SnowdenPlatformInterface(this, this)

            val handler = SnowdenCommandServerHandler(object : SnowdenCommandServerHandler.Callback {
                override fun onServiceReload() {
                    // Phase 2: re-emit the most recent configJson through startOrReloadService.
                    // For now, nag libbox to re-read the active profile.
                    try {
                        val server = commandServer ?: return
                        // We don't keep the original config after start; a reload
                        // without new content just renews the running box.
                        server.startOrReloadService("", OverrideOptions())
                    } catch (t: Throwable) {
                        Log.w(TAG, "onServiceReload ignored", t)
                    }
                }

                override fun onServiceStop() {
                    stopVpn()
                }
            })

            commandServer = Libbox.newCommandServer(handler, platformInterface)

            // Start the command-protocol listener (any Flutter-side CommandClient
            // can attach here). Safe to call even if we never use it directly.
            commandServer?.start()

            // Directly load and start the VPN service. This is the mobile path -
            // we already have the profile, no need to round-trip through a
            // CommandClient.
            val override = OverrideOptions().apply {
                autoRedirect = false
                includePackage = ListStringIterator(emptyList())
                excludePackage = ListStringIterator(listOf(packageName))
            }
            commandServer?.startOrReloadService(configJson, override)

            isRunning = true
            startForeground(NOTIFICATION_ID, buildNotification("snowden.system — VPN активен"))
        } catch (e: Exception) {
            Log.e(TAG, "startVpn failed", e)
            stopVpn()
        }
    }

    // -- Stop --------------------------------------------------------------

    private fun stopVpn() {
        if (commandServer == null && platformInterface == null) {
            // Nothing to do, but still tear down the foreground notification in case
            // the service was kept around without an active session.
            stopForeground(STOP_FOREGROUND_REMOVE)
            isRunning = false
            stopSelf()
            return
        }

        try {
            commandServer?.closeService()
        } catch (t: Throwable) {
            Log.w(TAG, "closeService failed", t)
        }
        try {
            commandServer?.close()
        } catch (t: Throwable) {
            Log.w(TAG, "commandServer.close failed", t)
        }

        commandServer = null
        platformInterface = null

        isRunning = false
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    // -- Setup options -----------------------------------------------------

    private fun applySetupOptionsOnce() {
        if (setupApplied) return

        val base = filesDir
        val options = SetupOptions().apply {
            basePath = base.absolutePath
            workingPath = base.absolutePath
            tempPath = cacheDir.absolutePath
            logMaxLines = 500
            debug = false

            // Random short-lived credentials for the command-protocol listener.
            // We don't currently expose the listener to external clients, but
            // having unique values avoids stale-process cross-talk after
            // a restart.
            commandServerListenPort = Libbox.availablePort(0)
            commandServerSecret = Libbox.randomHex(16).value
        }
        Libbox.setup(options)
        setupApplied = true
    }

    // -- Notification ------------------------------------------------------

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                NOTIFICATION_CHANNEL_ID,
                "Snowden VPN",
                NotificationManager.IMPORTANCE_LOW
            ).apply {
                description = "VPN connection status"
            }
            getSystemService(NotificationManager::class.java)
                ?.createNotificationChannel(channel)
        }
    }

    private fun buildNotification(text: String): Notification {
        val intent = Intent(this, MainActivity::class.java).apply {
            flags = Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP
        }
        val pendingIntent = PendingIntent.getActivity(
            this, 0, intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )

        return NotificationCompat.Builder(this, NOTIFICATION_CHANNEL_ID)
            .setContentTitle("snowden.system")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.ic_lock_idle_lock)
            .setContentIntent(pendingIntent)
            .setOngoing(true)
            .build()
    }
}

/**
 * Tiny helper that turns a Kotlin list into a libbox StringIterator.
 *
 * OverrideOptions includePackage/excludePackage are exposed through
 * io.nekohasekai.libbox.StringIterator so we adapt once and reuse.
 */
internal class ListStringIterator(private val items: List<String>) : StringIterator {
    private var index = 0
    override fun hasNext(): Boolean = index < items.size
    override fun len(): Int = items.size - index
    override fun next(): String = items[index++]
}
